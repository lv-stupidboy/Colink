import { app, ipcMain, BrowserWindow } from "electron";
import { spawn, execFile } from "child_process";
import { readFile, writeFile, mkdir } from "fs/promises";
import { existsSync } from "fs";
import { join, dirname } from "path";
import { homedir } from "os";
import type { DaemonStatus } from "../shared/daemon-types";

const DEFAULT_SERVER_PORT = 26305;
const POLL_INTERVAL_MS = 5_000;
const PREFS_PATH = join(homedir(), ".colink", "desktop_prefs.json");

const DEFAULT_PREFS = { autoStart: true, autoStop: true };

let statusPollTimer: ReturnType<typeof setInterval> | null = null;
let currentState: DaemonStatus["state"] = "installing_cli";
let getMainWindow: () => BrowserWindow | null = () => null;
let operationInProgress = false;
let cachedServerBinary: string | null | undefined = undefined;
let targetApiBaseUrl: string | null = null;
let serverProcess: ReturnType<typeof spawn> | null = null;

function sendStatus(status: DaemonStatus): void {
  const win = getMainWindow();
  win?.webContents.send("daemon:status", status);
}

interface HealthPayload {
  status?: string;
  version?: string;
  gitCommit?: string;
  buildTime?: string;
}

async function fetchHealthAtPort(port: number): Promise<HealthPayload | null> {
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 2_000);
    const res = await fetch(`http://127.0.0.1:${port}/health`, {
      signal: controller.signal,
    });
    clearTimeout(timeout);
    if (!res.ok) return null;
    return (await res.json()) as HealthPayload;
  } catch {
    return null;
  }
}

/**
 * Returns the path to the server binary bundled inside the Desktop app.
 * In dev: apps/desktop/resources/bin/colink-server.exe
 * In packaged: app.asar.unpacked/resources/bin/
 */
function bundledServerPath(): string {
  const binName = process.platform === "win32" ? "colink-server.exe" : "colink-server";
  return join(app.getAppPath(), "resources", "bin", binName).replace(
    "app.asar",
    "app.asar.unpacked"
  );
}

function findServerOnPath(): string | null {
  const candidates = process.platform === "win32" ? ["colink-server.exe"] : ["colink-server"];
  const paths = (process.env["PATH"] ?? "").split(process.platform === "win32" ? ";" : ":");
  for (const name of candidates) {
    for (const dir of paths) {
      const full = join(dir, name);
      if (existsSync(full)) return full;
    }
  }
  return null;
}

async function resolveServerBinary(): Promise<string | null> {
  if (cachedServerBinary !== undefined) return cachedServerBinary;

  const bundled = bundledServerPath();
  if (existsSync(bundled)) {
    console.log(`[daemon] using bundled server at ${bundled}`);
    cachedServerBinary = bundled;
    return bundled;
  }

  const onPath = findServerOnPath();
  if (onPath) {
    console.log(`[daemon] using server from PATH at ${onPath}`);
    cachedServerBinary = onPath;
    return onPath;
  }

  cachedServerBinary = null;
  return null;
}

async function fetchHealth(): Promise<DaemonStatus> {
  // Remote mode: no local daemon
  if (targetApiBaseUrl) {
    return { state: "remote", serverUrl: targetApiBaseUrl };
  }

  const data = await fetchHealthAtPort(DEFAULT_SERVER_PORT);

  if (!data || data.status !== "ok") {
    // Server not running - return current state or stopped
    const state = currentState === "starting" ? "starting" :
                  currentState === "cli_not_found" ? "cli_not_found" : "stopped";
    return { state };
  }

  // Server is running - always return running status
  currentState = "running";
  return {
    state: "running",
    version: data.version,
    gitCommit: data.gitCommit,
    buildTime: data.buildTime,
    serverUrl: `http://localhost:${DEFAULT_SERVER_PORT}`,
  };
}

async function loadPrefs(): Promise<typeof DEFAULT_PREFS> {
  try {
    const raw = await readFile(PREFS_PATH, "utf-8");
    const parsed = JSON.parse(raw);
    return { ...DEFAULT_PREFS, ...parsed };
  } catch {
    return { ...DEFAULT_PREFS };
  }
}

async function savePrefs(prefs: typeof DEFAULT_PREFS): Promise<void> {
  const dir = join(homedir(), ".colink");
  await mkdir(dir, { recursive: true });
  await writeFile(PREFS_PATH, JSON.stringify(prefs, null, 2), "utf-8");
}

async function withGuard<T>(fn: () => Promise<T>): Promise<T | { success: false; error: string }> {
  if (operationInProgress) {
    return { success: false, error: "Another daemon operation is in progress" };
  }
  operationInProgress = true;
  try {
    return await fn();
  } finally {
    operationInProgress = false;
  }
}

function desktopSpawnEnv(): NodeJS.ProcessEnv {
  return { ...process.env, COLINK_LAUNCHED_BY: "desktop" };
}

async function startDaemon(): Promise<{ success: boolean; error?: string }> {
  const bin = await resolveServerBinary();
  if (!bin) return { success: false, error: "Colink server is not installed" };

  const existing = await fetchHealthAtPort(DEFAULT_SERVER_PORT);
  if (existing?.status === "ok") {
    pollOnce();
    return { success: true };
  }

  currentState = "starting";
  sendStatus({ state: "starting" });

  // Find config file - support both package structures:
  // 1. New structure: packages/desktop/Colink.exe
  //    - config in ./data/configs/config.yaml (top level, copied by installer)
  //    - web in ./web/ (top level, copied by installer)
  //    - workDir = exeDir (packages/desktop)
  // 2. Old structure: Colink.exe and data/ in same directory
  //    - config in ./data/configs/config.yaml
  //    - workDir = exeDir
  // 3. Development mode: config in ../runtime/data/configs/ (for testing packages structure)
  const exeDir = dirname(app.getPath("exe"));

  let configPath: string;
  let workDir: string;

  // Priority: top-level config > runtime config
  // Top-level config is written by installer during installation
  const topLevelConfig = join(exeDir, "data", "configs", "config.yaml");
  const runtimeConfig = join(exeDir, "..", "runtime", "data", "configs", "config.yaml");

  if (existsSync(topLevelConfig)) {
    configPath = topLevelConfig;
    workDir = exeDir;
    console.log(`[daemon] using top-level config: ${configPath}, workDir: ${workDir}`);
  } else if (existsSync(runtimeConfig)) {
    // Fallback for development/testing with packages structure
    configPath = runtimeConfig;
    workDir = exeDir;
    console.log(`[daemon] using runtime config (dev mode): ${configPath}, workDir: ${workDir}`);
  } else {
    // Fall back to old flat structure
    configPath = join(exeDir, "data", "configs", "config.yaml");
    workDir = exeDir;
    console.log(`[daemon] using flat structure, config: ${configPath}, workDir: ${workDir}`);
  }

  // Set cwd to work directory (exeDir), where ./web/ should be located
  console.log(`[daemon] starting server: bin=${bin}, config=${configPath}, cwd=${workDir}`);
  serverProcess = spawn(bin, ["-config", configPath], {
    cwd: workDir,
    env: desktopSpawnEnv(),
    stdio: ["ignore", "ignore", "pipe"], // Capture stderr for debugging
    windowsHide: true, // Hide console window on Windows
  });

  // Log stderr for debugging
  if (serverProcess.stderr) {
    serverProcess.stderr.on("data", (data) => {
      console.error(`[daemon] server stderr: ${data.toString()}`);
    });
  }

  serverProcess.on("error", (err) => {
    console.error("[daemon] spawn error:", err);
    currentState = "stopped";
    sendStatus({ state: "stopped" });
  });

  serverProcess.on("exit", (code, signal) => {
    console.log(`[daemon] server process exited with code=${code}, signal=${signal}`);
    if (code !== 0 && currentState === "starting") {
      currentState = "stopped";
      sendStatus({ state: "stopped" });
    }
  });

  // Wait for server to become healthy (poll health endpoint)
  const maxWaitMs = 15_000;
  const pollIntervalMs = 500;
  let waited = 0;

  while (waited < maxWaitMs) {
    const health = await fetchHealthAtPort(DEFAULT_SERVER_PORT);
    if (health?.status === "ok") {
      currentState = "running";
      // Immediately send running status to renderer
      sendStatus({
        state: "running",
        version: health.version,
        gitCommit: health.gitCommit,
        buildTime: health.buildTime,
        serverUrl: `http://localhost:${DEFAULT_SERVER_PORT}`,
      });
      startPolling();
      return { success: true };
    }
    await new Promise((r) => setTimeout(r, pollIntervalMs));
    waited += pollIntervalMs;
  }

  // Timeout - server didn't start
  currentState = "stopped";
  sendStatus({ state: "stopped" });
  return { success: false, error: "Server did not become healthy within timeout" };
}

async function stopDaemon(): Promise<{ success: boolean; error?: string }> {
  currentState = "stopping";
  sendStatus({ state: "stopping" });

  // Kill process by port
  return new Promise((resolve) => {
    const killCmd = process.platform === "win32"
      ? `taskkill /f /im colink-server.exe`
      : `pkill -f colink-server`;
    execFile(
      process.platform === "win32" ? "cmd" : "sh",
      [process.platform === "win32" ? "/c" : "-c", killCmd],
      { timeout: 15_000 },
      (err) => {
        currentState = "stopped";
        sendStatus({ state: "stopped" });
        resolve(err ? { success: false, error: String(err) } : { success: true });
      }
    );
  });
}

async function restartDaemon(): Promise<{ success: boolean; error?: string }> {
  const stopResult = await stopDaemon();
  if (!stopResult.success) return stopResult;
  return startDaemon();
}

async function pollOnce(): Promise<void> {
  const status = await fetchHealth();
  currentState = status.state;
  sendStatus(status);
}

function startPolling(): void {
  if (statusPollTimer) return;
  pollOnce();
  statusPollTimer = setInterval(pollOnce, POLL_INTERVAL_MS);
}

function stopPolling(): void {
  if (statusPollTimer) {
    clearInterval(statusPollTimer);
    statusPollTimer = null;
  }
}

async function bootstrapServer(): Promise<void> {
  const bin = await resolveServerBinary();
  if (!bin) {
    currentState = "cli_not_found";
    sendStatus({ state: "cli_not_found" });
    return;
  }

  // Check if server is already running before setting state
  const health = await fetchHealthAtPort(DEFAULT_SERVER_PORT);
  if (health?.status === "ok") {
    currentState = "running";
    sendStatus({ state: "running", serverUrl: `http://localhost:${DEFAULT_SERVER_PORT}` });
  } else {
    currentState = "stopped";
    sendStatus({ state: "stopped" });
  }

  startPolling();
}

export function setupDaemonManager(
  windowGetter: () => BrowserWindow | null,
  initialTargetUrl: string | null
): void {
  getMainWindow = windowGetter;
  targetApiBaseUrl = initialTargetUrl;

  ipcMain.handle("daemon:start", () => withGuard(() => startDaemon()));
  ipcMain.handle("daemon:stop", () => withGuard(() => stopDaemon()));
  ipcMain.handle("daemon:restart", () => withGuard(() => restartDaemon()));
  ipcMain.handle("daemon:get-status", () => fetchHealth());
  ipcMain.handle("daemon:get-prefs", () => loadPrefs());
  ipcMain.handle("daemon:set-prefs", (_event, prefs: Partial<typeof DEFAULT_PREFS>) =>
    loadPrefs().then((cur) => {
      const merged = { ...cur, ...prefs };
      return savePrefs(merged).then(() => merged);
    }),
  );
  ipcMain.handle("daemon:auto-start", async () => {
    const prefs = await loadPrefs();
    if (!prefs.autoStart) return;
    // Remote mode: no auto-start
    if (targetApiBaseUrl) return;
    const health = await fetchHealth();
    if (health.state === "running") return;
    await startDaemon();
  });
  ipcMain.handle("daemon:set-target-url", (_event, url: string | null) => {
    targetApiBaseUrl = url;
    pollOnce();
  });

  currentState = "installing_cli";
  sendStatus({ state: "installing_cli" });
  void bootstrapServer();

  let isQuitting = false;
  app.on("before-quit", (event) => {
    if (isQuitting) return;
    stopPolling();

    // Always stop daemon on quit (regardless of prefs)
    // This ensures colink-server.exe is killed when desktop app closes
    isQuitting = true;
    event.preventDefault();

    stopDaemon().then(() => {
      app.quit();
    }).catch(() => {
      app.quit();
    });
  });
}