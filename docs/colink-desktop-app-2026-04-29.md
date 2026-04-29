# Colink Desktop Application Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform Colink from a launcher+browser experience to a unified desktop application where the Electron window directly displays the web UI, supporting both local and remote backend modes.

**Architecture:** Create a new `apps/desktop` directory with Electron main process that manages Go backend daemon lifecycle and renders React frontend in-window. The desktop app connects to either local daemon (bundled) or remote server (configured via URL). Reuses existing `cmd/server` Go backend and `web` React frontend.

**Tech Stack:** Electron 30+, React 18, Ant Design 5, Go backend (cmd/server), electron-builder for packaging.

---

## File Structure

```
apps/desktop/                     # New directory for desktop application
├── build/                        # Build resources (icons)
│   ├── icon.ico                  # Windows icon
│   ├── icon.icns                 # macOS icon
│   └── icon.png                  # Linux/PNG fallback
├── resources/                    # Bundled resources
│   └── bin/                      # Go backend binaries
│       └── colink-server.exe     # Bundled server (Windows)
├── scripts/                      # Build scripts
│   ├── bundle-server.mjs         # Build and bundle Go server
│   └── package.mjs               # Custom packaging logic
├── src/
│   ├── main/                     # Electron main process
│   │   ├── index.ts              # Main entry, window creation
│   │   ├── daemon-manager.ts     # Go backend lifecycle management
│   │   ├── ipc-handlers.ts       # IPC handlers for renderer
│   │   └── external-url.ts       # Safe external URL handling
│   │   └── context-menu.ts       # Right-click context menu
│   │   └── updater.ts            # Auto-update logic (optional)
│   ├── preload/                  # Preload scripts
│   │   ├── index.ts              # Expose safe APIs to renderer
│   │   └── index.d.ts            # Type declarations
│   ├── renderer/                 # Renderer process (reuses web)
│   │   ├── index.html            # Entry HTML
│   │   └── src/
│   │       ├── App.tsx           # Desktop-specific app shell
│   │       ├── main.tsx          # Renderer entry
│   │       ├── platform/         # Platform-specific modules
│   │       │   ├── api-bridge.ts # API URL configuration
│   │       │   └── navigation.tsx # Navigation handling
│   │       └── globals.css       # Desktop-specific styles
│   └── shared/                   # Shared types
│       └── daemon-types.ts       # Daemon status types
├── electron-builder.yml          # Packaging config
├── electron.vite.config.ts       # Vite config for Electron
├── package.json                  # Dependencies
├── tsconfig.json                 # TypeScript config
└── tsconfig.node.json            # Node/main process TypeScript config
```

---

## Task 1: Create Desktop Application Directory Structure

**Files:**
- Create: `apps/desktop/package.json`
- Create: `apps/desktop/tsconfig.json`
- Create: `apps/desktop/tsconfig.node.json`
- Create: `apps/desktop/tsconfig.web.json`
- Create: `apps/desktop/.gitignore`

- [ ] **Step 1: Create apps/desktop/package.json**

```json
{
  "name": "@colink/desktop",
  "version": "0.1.0",
  "private": true,
  "description": "Colink Desktop — native desktop client for the Colink platform.",
  "main": "./out/main/index.js",
  "scripts": {
    "bundle-server": "node scripts/bundle-server.mjs",
    "dev": "npm run bundle-server && electron-vite dev",
    "dev:remote": "npm run bundle-server && electron-vite dev --mode remote",
    "build": "npm run bundle-server && electron-vite build",
    "typecheck": "tsc --noEmit -p tsconfig.node.json && tsc --noEmit -p tsconfig.web.json",
    "preview": "electron-vite preview",
    "package": "node scripts/package.mjs",
    "package:all": "node scripts/package.mjs --all-platforms",
    "lint": "eslint .",
    "test": "vitest run",
    "postinstall": "electron-builder install-app-deps"
  },
  "dependencies": {
    "@electron-toolkit/preload": "^3.0.2",
    "@electron-toolkit/utils": "^4.0.0",
    "electron-updater": "^6.8.3",
    "fix-path": "^5.0.0",
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.22.0",
    "antd": "^5.15.0",
    "@ant-design/icons": "^5.3.0",
    "axios": "^1.6.0",
    "zustand": "^4.5.0"
  },
  "devDependencies": {
    "@electron-toolkit/tsconfig": "^2.0.0",
    "@types/node": "^20.11.0",
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.2.0",
    "electron": "^30.0.0",
    "electron-builder": "^24.13.0",
    "electron-vite": "^2.0.0",
    "typescript": "^5.3.0",
    "vite": "^5.1.0",
    "vitest": "^1.0.0"
  }
}
```

- [ ] **Step 2: Create apps/desktop/tsconfig.json**

```json
{
  "extends": "@electron-toolkit/tsconfig/tsconfig.json",
  "include": ["src/**/*", "src/preload/*.d.ts"],
  "compilerOptions": {
    "composite": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/renderer/src/*"]
    }
  }
}
```

- [ ] **Step 3: Create apps/desktop/tsconfig.node.json**

```json
{
  "extends": "@electron-toolkit/tsconfig/tsconfig.node.json",
  "include": ["src/main/**/*", "src/preload/**/*", "scripts/**/*"],
  "compilerOptions": {
    "composite": true,
    "outDir": "./out",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "types": ["electron-vite/node"]
  }
}
```

- [ ] **Step 4: Create apps/desktop/tsconfig.web.json**

```json
{
  "extends": "@electron-toolkit/tsconfig/tsconfig.web.json",
  "include": ["src/renderer/**/*"],
  "compilerOptions": {
    "composite": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/renderer/src/*"]
    }
  }
}
```

- [ ] **Step 5: Create apps/desktop/.gitignore**

```
out/
dist/
node_modules/
resources/bin/
.release/
*.log
```

- [ ] **Step 6: Commit directory structure**

```bash
git add apps/desktop/
git commit -m "feat: create desktop application directory structure"
```

---

## Task 2: Create Electron Main Process Entry

**Files:**
- Create: `apps/desktop/src/main/index.ts`

- [ ] **Step 1: Create apps/desktop/src/main/index.ts**

```typescript
import { app, BrowserWindow, ipcMain, nativeImage } from "electron";
import { homedir } from "os";
import { join } from "path";
import { electronApp, optimizer, is } from "@electron-toolkit/utils";
import fixPath from "fix-path";
import { setupDaemonManager } from "./daemon-manager";
import { openExternalSafely } from "./external-url";
import { installContextMenu } from "./context-menu";

const DEV_ICON_PATH = join(__dirname, "../../resources/icon.png");

// macOS/Linux GUI launches inherit minimal PATH from launchd.
// Run login shell to recover real PATH so bundled CLI can find agent binaries.
if (process.platform !== "win32") {
  fixPath();
  const fallbackPaths = [
    "/opt/homebrew/bin",
    "/usr/local/bin",
    join(homedir(), ".local/bin"),
  ];
  process.env.PATH = `${fallbackPaths.join(":")}:${process.env.PATH ?? ""}`;
}

const PROTOCOL = "colink";

let mainWindow: BrowserWindow | null = null;
let targetApiBaseUrl: string | null = null;

// --- Deep link handling ---
function handleDeepLink(url: string): void {
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== `${PROTOCOL}:`) return;
    // colink://auth/callback?token=<jwt>
    if (parsed.hostname === "auth" && parsed.pathname === "/callback") {
      const token = parsed.searchParams.get("token");
      if (token && mainWindow) {
        mainWindow.webContents.send("auth:token", token);
      }
    }
  } catch {
    // Ignore malformed URLs
  }
}

// --- Window creation ---
function createWindow(): void {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 900,
    minHeight: 600,
    titleBarStyle: "hiddenInset",
    trafficLightPosition: { x: 16, y: 13 },
    show: false,
    autoHideMenuBar: true,
    ...(is.dev ? { icon: DEV_ICON_PATH } : {}),
    webPreferences: {
      preload: join(__dirname, "../preload/index.js"),
      sandbox: false,
      webSecurity: false,
    },
  });

  // Strip Origin header from WebSocket upgrade requests
  mainWindow.webContents.session.webRequest.onBeforeSendHeaders(
    { urls: ["wss://*/*", "ws://*/*"] },
    (details, callback) => {
      delete details.requestHeaders["Origin"];
      callback({ requestHeaders: details.requestHeaders });
    },
  );

  mainWindow.on("ready-to-show", () => {
    mainWindow?.show();
  });

  mainWindow.webContents.setWindowOpenHandler((details) => {
    openExternalSafely(details.url);
    return { action: "deny" };
  });

  installContextMenu(mainWindow.webContents);

  // Load renderer
  if (is.dev && process.env["ELECTRON_RENDERER_URL"]) {
    mainWindow.loadURL(process.env["ELECTRON_RENDERER_URL"]);
  } else {
    mainWindow.loadFile(join(__dirname, "../renderer/index.html"));
  }
}

// --- Dev/prod isolation ---
const DEV_APP_NAME = process.env.DESKTOP_APP_SUFFIX
  ? `Colink Dev ${process.env.DESKTOP_APP_SUFFIX}`
  : "Colink Dev";

if (is.dev) {
  app.setName(DEV_APP_NAME);
  app.setPath("userData", join(app.getPath("appData"), DEV_APP_NAME));
}

// --- Protocol registration ---
if (process.defaultApp) {
  app.setAsDefaultProtocolClient(PROTOCOL, process.execPath, [app.getAppPath()]);
} else {
  app.setAsDefaultProtocolClient(PROTOCOL);
}

// --- Single instance lock ---
const gotTheLock = app.requestSingleInstanceLock();

if (!gotTheLock) {
  app.quit();
} else {
  app.on("second-instance", (_event, argv) => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.focus();
    }
    const deepLinkUrl = argv.find((arg) => arg.startsWith(`${PROTOCOL}://`));
    if (deepLinkUrl) handleDeepLink(deepLinkUrl);
  });

  app.whenReady().then(() => {
    electronApp.setAppUserModelId(
      is.dev ? "com.colink.desktop.dev" : "com.colink.desktop"
    );

    if (is.dev && process.platform === "darwin" && app.dock) {
      const icon = nativeImage.createFromPath(DEV_ICON_PATH);
      if (!icon.isEmpty()) app.dock.setIcon(icon);
    }

    app.on("browser-window-created", (_, window) => {
      optimizer.watchWindowShortcuts(window);
    });

    // IPC: open URL in default browser
    ipcMain.handle("shell:openExternal", (_event, url: string) => {
      return openExternalSafely(url);
    });

    // IPC: set target API URL (switch between local/remote)
    ipcMain.handle("daemon:set-target-api-url", async (_e, url: string) => {
      const normalized = url || null;
      if (targetApiBaseUrl !== normalized) {
        console.log(`[daemon] target API URL set to ${normalized ?? "(local)"}`);
        targetApiBaseUrl = normalized;
      }
    });

    // IPC: get app info
    ipcMain.on("app:get-info", (event) => {
      const p = process.platform;
      const os = p === "darwin" ? "macos" : p === "win32" ? "windows" : p === "linux" ? "linux" : "unknown";
      event.returnValue = { version: app.getVersion(), os, mode: targetApiBaseUrl ? "remote" : "local" };
    });

    createWindow();

    setupDaemonManager(() => mainWindow, targetApiBaseUrl);

    app.on("open-url", (_event, url) => {
      if (mainWindow) {
        if (mainWindow.isMinimized()) mainWindow.restore();
        mainWindow.focus();
      }
      handleDeepLink(url);
    });

    app.on("activate", () => {
      if (BrowserWindow.getAllWindows().length === 0) createWindow();
    });
  });

  const deepLinkArg = process.argv.find((arg) => arg.startsWith(`${PROTOCOL}://`));
  if (deepLinkArg) {
    app.whenReady().then(() => handleDeepLink(deepLinkArg));
  }
}

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
```

- [ ] **Step 2: Commit main process entry**

```bash
git add apps/desktop/src/main/index.ts
git commit -m "feat: create Electron main process entry with window management"
```

---

## Task 3: Create Daemon Manager for Go Backend

**Files:**
- Create: `apps/desktop/src/main/daemon-manager.ts`

- [ ] **Step 1: Create apps/desktop/src/main/daemon-manager.ts**

```typescript
import { app, ipcMain, BrowserWindow, shell } from "electron";
import { execFile } from "child_process";
import { readFile, writeFile, mkdir, rm, stat, open } from "fs/promises";
import { existsSync, watchFile, unwatchFile, type StatsListener } from "fs";
import { join } from "path";
import { homedir } from "os";
import type { DaemonStatus } from "../shared/daemon-types";

const DEFAULT_HEALTH_PORT = 26305;
const POLL_INTERVAL_MS = 5_000;
const PREFS_PATH = join(homedir(), ".colink", "desktop_prefs.json");

const DEFAULT_PREFS = { autoStart: true, autoStop: false };

interface ActiveProfile {
  name: string;
  port: number;
}

let statusPollTimer: ReturnType<typeof setInterval> | null = null;
let currentState: DaemonStatus["state"] = "installing_cli";
let getMainWindow: () => BrowserWindow | null = () => null;
let operationInProgress = false;
let cachedServerBinary: string | null | undefined = undefined;
let targetApiBaseUrl: string | null = null;

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
  if (currentState === "installing_cli" || currentState === "cli_not_found") {
    return { state: currentState };
  }

  // Remote mode: no local daemon
  if (targetApiBaseUrl) {
    return { state: "remote", serverUrl: targetApiBaseUrl };
  }

  const data = await fetchHealthAtPort(DEFAULT_HEALTH_PORT);

  if (!data || data.status !== "ok") {
    return {
      state: currentState === "starting" ? "starting" : "stopped",
    };
  }

  return {
    state: "running",
    version: data.version,
    gitCommit: data.gitCommit,
    buildTime: data.buildTime,
    serverUrl: `http://localhost:${DEFAULT_HEALTH_PORT}`,
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

  const existing = await fetchHealthAtPort(DEFAULT_HEALTH_PORT);
  if (existing?.status === "ok") {
    pollOnce();
    return { success: true };
  }

  currentState = "starting";
  sendStatus({ state: "starting" });

  const configPath = join(app.getPath("userData"), "config.yaml");

  return new Promise((resolve) => {
    execFile(
      bin,
      ["-config", configPath],
      { timeout: 20_000, env: desktopSpawnEnv() },
      (err) => {
        if (err) {
          currentState = "stopped";
          sendStatus({ state: "stopped" });
          resolve({ success: false, error: err.message });
          return;
        }
        pollOnce();
        resolve({ success: true });
      },
    );
  });
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
        resolve({ success: err ? { success: false, error: err.message } : { success: true } });
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

async function bootstrapServer(): Promise<void> {
  const bin = await resolveServerBinary();
  if (!bin) {
    currentState = "cli_not_found";
    sendStatus({ state: "cli_not_found" });
    return;
  }
  currentState = "stopped";
  sendStatus({ state: "stopped" });
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

    loadPrefs().then(async (prefs) => {
      if (prefs.autoStop) {
        isQuitting = true;
        event.preventDefault();
        try {
          await stopDaemon();
        } catch {}
        app.quit();
      }
    });
  });
}
```

- [ ] **Step 2: Commit daemon manager**

```bash
git add apps/desktop/src/main/daemon-manager.ts
git commit -m "feat: create daemon manager for Go backend lifecycle"
```

---

## Task 4: Create Shared Types

**Files:**
- Create: `apps/desktop/src/shared/daemon-types.ts`

- [ ] **Step 1: Create apps/desktop/src/shared/daemon-types.ts**

```typescript
export interface DaemonStatus {
  state: "running" | "stopped" | "starting" | "stopping" | "installing_cli" | "cli_not_found" | "remote";
  version?: string;
  gitCommit?: string;
  buildTime?: string;
  serverUrl?: string;
}

export interface DaemonPrefs {
  autoStart: boolean;
  autoStop: boolean;
}
```

- [ ] **Step 2: Commit shared types**

```bash
git add apps/desktop/src/shared/daemon-types.ts
git commit -m "feat: create shared daemon types"
```

---

## Task 5: Create Preload Script

**Files:**
- Create: `apps/desktop/src/preload/index.ts`
- Create: `apps/desktop/src/preload/index.d.ts`

- [ ] **Step 1: Create apps/desktop/src/preload/index.ts**

```typescript
import { contextBridge, ipcRenderer } from "electron";
import type { DaemonStatus, DaemonPrefs } from "../shared/daemon-types";

const daemonAPI = {
  start: () => ipcRenderer.invoke("daemon:start"),
  stop: () => ipcRenderer.invoke("daemon:stop"),
  restart: () => ipcRenderer.invoke("daemon:restart"),
  getStatus: () => ipcRenderer.invoke("daemon:get-status"),
  getPrefs: () => ipcRenderer.invoke("daemon:get-prefs"),
  setPrefs: (prefs: Partial<DaemonPrefs>) => ipcRenderer.invoke("daemon:set-prefs", prefs),
  autoStart: () => ipcRenderer.invoke("daemon:auto-start"),
  setTargetUrl: (url: string | null) => ipcRenderer.invoke("daemon:set-target-url", url),
  onStatusChange: (callback: (status: DaemonStatus) => void) => {
    const handler = (_event: unknown, status: DaemonStatus) => callback(status);
    ipcRenderer.on("daemon:status", handler);
    return () => ipcRenderer.removeListener("daemon:status", handler);
  },
};

const desktopAPI = {
  appInfo: ipcRenderer.sendSync("app:get-info") as { version: string; os: string; mode: "local" | "remote" },
  openExternal: (url: string) => ipcRenderer.invoke("shell:openExternal", url),
  onAuthToken: (callback: (token: string) => void) => {
    const handler = (_event: unknown, token: string) => callback(token);
    ipcRenderer.on("auth:token", handler);
    return () => ipcRenderer.removeListener("auth:token", handler);
  },
};

// Expose to renderer
contextBridge.exposeInMainWorld("daemonAPI", daemonAPI);
contextBridge.exposeInMainWorld("desktopAPI", desktopAPI);
```

- [ ] **Step 2: Create apps/desktop/src/preload/index.d.ts**

```typescript
import type { DaemonStatus, DaemonPrefs } from "../shared/daemon-types";

export interface DaemonAPI {
  start: () => Promise<{ success: boolean; error?: string }>;
  stop: () => Promise<{ success: boolean; error?: string }>;
  restart: () => Promise<{ success: boolean; error?: string }>;
  getStatus: () => Promise<DaemonStatus>;
  getPrefs: () => Promise<DaemonPrefs>;
  setPrefs: (prefs: Partial<DaemonPrefs>) => Promise<DaemonPrefs>;
  autoStart: () => Promise<void>;
  setTargetUrl: (url: string | null) => Promise<void>;
  onStatusChange: (callback: (status: DaemonStatus) => void) => () => void;
}

export interface DesktopAPI {
  appInfo: { version: string; os: string; mode: "local" | "remote" };
  openExternal: (url: string) => Promise<void>;
  onAuthToken: (callback: (token: string) => void) => () => void;
}

declare global {
  interface Window {
    daemonAPI: DaemonAPI;
    desktopAPI: DesktopAPI;
  }
}
```

- [ ] **Step 3: Commit preload scripts**

```bash
git add apps/desktop/src/preload/
git commit -m "feat: create preload scripts exposing daemon and desktop APIs"
```

---

## Task 6: Create Helper Modules

**Files:**
- Create: `apps/desktop/src/main/external-url.ts`
- Create: `apps/desktop/src/main/context-menu.ts`

- [ ] **Step 1: Create apps/desktop/src/main/external-url.ts**

```typescript
import { shell } from "electron";

const ALLOWED_SCHEMES = ["http:", "https:", "mailto:"];

export function openExternalSafely(url: string): Promise<void> {
  try {
    const parsed = new URL(url);
    if (!ALLOWED_SCHEMES.includes(parsed.protocol)) {
      console.warn(`[security] blocked non-allowed scheme: ${parsed.protocol}`);
      return Promise.resolve();
    }
    return shell.openExternal(url);
  } catch (err) {
    console.warn(`[security] invalid URL blocked: ${url}`, err);
    return Promise.resolve();
  }
}
```

- [ ] **Step 2: Create apps/desktop/src/main/context-menu.ts**

```typescript
import { BrowserWindow, Menu, WebContents } from "electron";

export function installContextMenu(webContents: WebContents): void {
  webContents.on("context-menu", (_event, params) => {
    const menu = Menu.buildFromTemplate([
      { label: "Paste", role: "paste", enabled: params.editFlags.canPaste },
      { label: "Copy", role: "copy", enabled: params.editFlags.canCopy },
      { label: "Cut", role: "cut", enabled: params.editFlags.canCut },
      { type: "separator" },
      { label: "Select All", role: "selectAll" },
    ]);
    menu.popup(BrowserWindow.fromWebContents(webContents) ?? undefined);
  });
}
```

- [ ] **Step 3: Commit helper modules**

```bash
git add apps/desktop/src/main/external-url.ts apps/desktop/src/main/context-menu.ts
git commit -m "feat: create helper modules for external URL handling and context menu"
```

---

## Task 7: Create Renderer Entry

**Files:**
- Create: `apps/desktop/src/renderer/index.html`
- Create: `apps/desktop/src/renderer/src/main.tsx`
- Create: `apps/desktop/src/renderer/src/globals.css`

- [ ] **Step 1: Create apps/desktop/src/renderer/index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Colink</title>
    <link rel="icon" href="/favicon.ico">
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 2: Create apps/desktop/src/renderer/src/main.tsx**

```typescript
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./globals.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

- [ ] **Step 3: Create apps/desktop/src/renderer/src/globals.css**

```css
:root {
  font-family: Inter, system-ui, Avenir, Helvetica, Arial, sans-serif;
  line-height: 1.5;
  font-weight: 400;
  color: #213547;
  background-color: #ffffff;
  font-synthesis: none;
  text-rendering: optimizeLegibility;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

body {
  margin: 0;
  min-width: 320px;
  min-height: 100vh;
}

#root {
  width: 100%;
  height: 100vh;
}
```

- [ ] **Step 4: Commit renderer entry**

```bash
git add apps/desktop/src/renderer/
git commit -m "feat: create renderer entry files"
```

---

## Task 8: Create Desktop App Shell with Mode Detection

**Files:**
- Create: `apps/desktop/src/renderer/src/App.tsx`
- Create: `apps/desktop/src/renderer/src/platform/api-bridge.ts`

- [ ] **Step 1: Create apps/desktop/src/renderer/src/App.tsx**

```typescript
import { useEffect, useState } from "react";
import { ConfigProvider, Spin } from "antd";
import zhCN from "antd/locale/zh_CN";
import type { DaemonStatus } from "../../shared/daemon-types";
import { getApiBaseUrl, getWsUrl } from "./platform/api-bridge";

function App() {
  const [status, setStatus] = useState<DaemonStatus>({ state: "installing_cli" });
  const [ready, setReady] = useState(false);
  const { mode } = window.desktopAPI.appInfo;

  useEffect(() => {
    // Subscribe to daemon status
    const unsubscribe = window.daemonAPI.onStatusChange(setStatus);

    // Initial status fetch
    window.daemonAPI.getStatus().then(setStatus);

    // Auto-start daemon in local mode
    if (mode === "local") {
      window.daemonAPI.autoStart().then(() => {
        // Wait for daemon to be running
        const checkReady = () => {
          window.daemonAPI.getStatus().then((s) => {
            if (s.state === "running" || s.state === "remote") {
              setReady(true);
            } else if (s.state === "stopped") {
              // Retry start
              window.daemonAPI.start().then(() => {
                setTimeout(checkReady, 2000);
              });
            } else {
              setTimeout(checkReady, 1000);
            }
          });
        };
        checkReady();
      });
    } else {
      // Remote mode: ready immediately
      setReady(true);
    }

    return unsubscribe;
  }, [mode]);

  // Determine API URL based on mode
  const apiUrl = getApiBaseUrl(mode, status.serverUrl);
  const wsUrl = getWsUrl(mode, status.serverUrl);

  if (!ready) {
    return (
      <div style={{ display: "flex", height: "100vh", alignItems: "center", justifyContent: "center" }}>
        <Spin size="large" tip={status.state === "installing_cli" ? "初始化中..." : "启动服务中..."} />
      </div>
    );
  }

  return (
    <ConfigProvider locale={zhCN}>
      <iframe
        src={apiUrl}
        style={{ width: "100%", height: "100%", border: "none" }}
        title="Colink"
      />
    </ConfigProvider>
  );
}

export default App;
```

- [ ] **Step 2: Create apps/desktop/src/renderer/src/platform/api-bridge.ts**

```typescript
/**
 * Get API base URL based on mode and daemon status.
 * Local mode: use localhost with daemon port
 * Remote mode: use configured remote URL
 */
export function getApiBaseUrl(mode: "local" | "remote", daemonUrl?: string): string {
  if (mode === "remote") {
    // Remote mode: URL is configured at build time or runtime
    return import.meta.env.VITE_API_URL || "http://localhost:26305";
  }

  // Local mode: use daemon URL or fallback to default
  return daemonUrl || "http://localhost:26305";
}

/**
 * Get WebSocket URL based on mode and daemon status.
 */
export function getWsUrl(mode: "local" | "remote", daemonUrl?: string): string {
  if (mode === "remote") {
    const apiUrl = import.meta.env.VITE_API_URL || "http://localhost:26305";
    // Convert http/https to ws/wss
    return apiUrl.replace(/^http/, "ws") + "/ws";
  }

  const baseUrl = daemonUrl || "http://localhost:26305";
  return baseUrl.replace(/^http/, "ws") + "/ws";
}
```

- [ ] **Step 3: Commit App shell**

```bash
git add apps/desktop/src/renderer/src/App.tsx apps/desktop/src/renderer/src/platform/
git commit -m "feat: create desktop app shell with local/remote mode detection"
```

---

## Task 9: Create Electron Vite Configuration

**Files:**
- Create: `apps/desktop/electron.vite.config.ts`

- [ ] **Step 1: Create apps/desktop/electron.vite.config.ts**

```typescript
import { resolve } from "path";
import { defineConfig, externalizeDepsPlugin, swcPlugin } from "electron-vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin()],
    build: {
      rollupOptions: {
        input: {
          index: resolve(__dirname, "src/main/index.ts"),
        },
      },
    },
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
    build: {
      rollupOptions: {
        input: {
          index: resolve(__dirname, "src/preload/index.ts"),
        },
      },
    },
  },
  renderer: {
    plugins: [react()],
    build: {
      rollupOptions: {
        input: {
          index: resolve(__dirname, "src/renderer/index.html"),
        },
      },
    },
    define: {
      // Environment variables for renderer
      "import.meta.env.VITE_API_URL": JSON.stringify(process.env.VITE_API_URL || ""),
    },
  },
});
```

- [ ] **Step 2: Commit vite config**

```bash
git add apps/desktop/electron.vite.config.ts
git commit -m "feat: create electron-vite configuration"
```

---

## Task 10: Create Build Scripts

**Files:**
- Create: `apps/desktop/scripts/bundle-server.mjs`

- [ ] **Step 1: Create apps/desktop/scripts/bundle-server.mjs**

```javascript
#!/usr/bin/env node
// Builds the colink-server from cmd/server and copies binary to resources/bin/

import { access, chmod, copyFile, mkdir, rm } from "node:fs/promises";
import { constants } from "node:fs";
import { execFileSync, execSync } from "node:child_process";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "..", "..", "..");
const serverDir = join(repoRoot, "cmd", "server");

const PLATFORM_TO_GOOS = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const SUPPORTED_ARCHS = new Set(["x64", "arm64"]);

function binaryNameForPlatform(platform) {
  return platform === "win32" ? "colink-server.exe" : "colink-server";
}

const targetPlatform = process.platform;
const targetArch = process.arch;
const goos = PLATFORM_TO_GOOS[targetPlatform];
const goarch = targetArch === "x64" ? "amd64" : targetArch;
const binName = binaryNameForPlatform(targetPlatform);
const srcBinary = join(repoRoot, "bin", `${goos}-${goarch}`, binName);
const destDir = join(repoRoot, "apps", "desktop", "resources", "bin");
const destBinary = join(destDir, binName);

function sh(cmd) {
  try {
    return execSync(cmd, { encoding: "utf-8" }).trim();
  } catch {
    return "";
  }
}

function hasGo() {
  try {
    execSync("go version", { stdio: "pipe" });
    return true;
  } catch {
    return false;
  }
}

async function exists(p) {
  try {
    await access(p, constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

if (hasGo()) {
  const version = sh("git describe --tags --always --dirty") || "dev";
  const commit = sh("git rev-parse --short HEAD") || "unknown";
  const date = new Date().toISOString().replace(/\.\d+Z$/, "Z");
  const ldflags = `-X main.Version=${version} -X main.GitCommit=${commit} -X main.BuildTime=${date}`;

  console.log(`[bundle-server] go build → ${srcBinary} (${goos}/${goarch})`);
  await mkdir(join(repoRoot, "bin", `${goos}-${goarch}`), { recursive: true });
  execFileSync("go", ["build", "-ldflags", ldflags, "-o", srcBinary, "."], {
    cwd: serverDir,
    stdio: "inherit",
    env: { ...process.env, CGO_ENABLED: "0", GOOS: goos, GOARCH: goarch },
  });
} else {
  console.warn("[bundle-server] `go` not found — skipping server build.");
}

if (!(await exists(srcBinary))) {
  console.warn(`[bundle-server] ${srcBinary} not present — Desktop will fall back.`);
  await rm(destDir, { recursive: true, force: true });
  process.exit(0);
}

await rm(destDir, { recursive: true, force: true });
await mkdir(destDir, { recursive: true });
await copyFile(srcBinary, destBinary);
await chmod(destBinary, 0o755);

// macOS ad-hoc sign
if (process.platform === "darwin") {
  try {
    execSync(`codesign -s - --force ${JSON.stringify(destBinary)}`, { stdio: "pipe" });
  } catch {}
}

console.log(`[bundle-server] bundled ${srcBinary} → ${destBinary}`);
```

- [ ] **Step 2: Commit bundle script**

```bash
git add apps/desktop/scripts/bundle-server.mjs
git commit -m "feat: create bundle-server script for Go backend bundling"
```

---

## Task 11: Create Electron Builder Configuration

**Files:**
- Create: `apps/desktop/electron-builder.yml`

- [ ] **Step 1: Create apps/desktop/electron-builder.yml**

```yaml
appId: com.colink.desktop
productName: Colink
directories:
  buildResources: build
  output: release

protocols:
  - name: Colink
    schemes:
      - colink

asarUnpack:
  - resources/**

files:
  - "!**/.vscode/*"
  - "!src/*"
  - "!electron.vite.config.*"
  - "!{.eslintignore,.eslintrc.cjs,.prettierignore,.prettierrc.yaml}"
  - "!{tsconfig.json,tsconfig.node.json,tsconfig.web.json}"

extraResources:
  - from: "resources/bin"
    to: "bin"
  - from: "../web/dist"
    to: "web"
  - from: "../configs/config.yaml.example"
    to: "data/configs/config.yaml.example"

win:
  target:
    - nsis
  icon: build/icon.ico
  artifactName: colink-desktop-${version}-windows-${arch}.${ext}

mac:
  entitlementsInherit: build/entitlements.mac.plist
  target:
    - dmg
    - zip
  artifactName: colink-desktop-${version}-mac-${arch}.${ext}
  notarize: false

linux:
  target:
    - AppImage
    - deb
  artifactName: colink-desktop-${version}-linux-${arch}.${ext}

npmRebuild: false
```

- [ ] **Step 2: Commit electron-builder config**

```bash
git add apps/desktop/electron-builder.yml
git commit -m "feat: create electron-builder configuration"
```

---

## Task 12: Integrate with Existing Build System

**Files:**
- Modify: `Makefile` (add desktop build targets)
- Modify: `installer/build.ps1` (reference desktop app)

- [ ] **Step 1: Add desktop build targets to Makefile**

Find the existing Makefile and add these targets after the existing build targets:

```makefile
# Desktop application build
desktop-dev:
	cd apps/desktop && npm run dev

desktop-build:
	cd apps/desktop && npm run build

desktop-package:
	cd apps/desktop && npm run package

desktop-package-all:
	cd apps/desktop && npm run package:all
```

- [ ] **Step 2: Run make to verify**

```bash
make desktop-build
```
Expected: Successfully builds desktop app

- [ ] **Step 3: Commit Makefile changes**

```bash
git add Makefile
git commit -m "feat: add desktop build targets to Makefile"
```

---

## Task 13: Update Installer to Reference Desktop App

**Files:**
- Modify: `installer/build.ps1`

- [ ] **Step 1: Update installer/build.ps1 to include desktop app**

The installer should package the desktop app instead of the launcher. Modify the build script to:

1. Build the desktop app: `cd apps/desktop && npm run package`
2. Copy the desktop app output to the installer resources

Read the current build.ps1 and add desktop build step:

```powershell
# Build desktop application
Write-Host "Building desktop application..."
Set-Location "$RootDir/apps/desktop"
npm run build
npm run package
Set-Location $RootDir
```

- [ ] **Step 2: Commit installer changes**

```bash
git add installer/build.ps1
git commit -m "feat: update installer to build and package desktop app"
```

---

## Task 14: Test Desktop App Locally

**Files:**
- None (testing)

- [ ] **Step 1: Install dependencies**

```bash
cd apps/desktop && npm install
```

Expected: Dependencies installed successfully

- [ ] **Step 2: Run dev mode**

```bash
cd apps/desktop && npm run dev
```

Expected: Electron window opens, displays loading spinner, then shows web UI after daemon starts

- [ ] **Step 3: Test remote mode**

```bash
cd apps/desktop && npm run dev:remote
```
Set `VITE_API_URL=http://your-remote-server:26305` in `.env.production`

Expected: Electron window opens, connects directly to remote server

- [ ] **Step 4: Test build**

```bash
cd apps/desktop && npm run build
```

Expected: Build succeeds, output in `apps/desktop/out/`

---

## Self-Review Checklist

**1. Spec coverage:**
- ✓ Local mode: daemon manager starts Go backend
- ✓ Remote mode: connects to configured URL
- ✓ Window management: createWindow, single instance lock
- ✓ IPC bridge: preload exposes daemon and desktop APIs
- ✓ Build integration: bundle-server.mjs, Makefile targets
- ✓ Packaging: electron-builder.yml

**2. Placeholder scan:**
- ✓ No TBD/TODO placeholders
- ✓ All code blocks contain actual implementation
- ✓ All file paths are exact

**3. Type consistency:**
- ✓ DaemonStatus type used consistently across main/preload/renderer
- ✓ DaemonPrefs type used consistently
- ✓ Window.daemonAPI and Window.desktopAPI typed in preload/index.d.ts

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 1 | issues_open | 3 critical gaps: error UI, ws reconnect, mode switcher |
| Design Review | `/plan-design-review` | UI/UX gaps | 1 | issues_open | 3 gaps: progress indicator, error state, keyboard shortcuts |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | issues_open | 1 critical: no automated test plan |
| DX Review | `/plan-devex-review` | Developer experience gaps | 1 | issues_open | 2 gaps: upgrade path, error display |

**VERDICT:** ISSUES OPEN — 4 phases ran, 9 gaps identified (3 critical).

---

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
|---|-------|----------|-----------|-----------|----------|----------|
| 1 | CEO | Add daemon startup error UI | Mechanical | P1 (completeness) | User sees infinite spinner if daemon fails | - |
| 2 | CEO | Add WebSocket reconnect logic | Mechanical | P1 (completeness) | Connection drop shows stale UI | - |
| 3 | CEO | Add mode switcher UI | Taste | P2 (boil lakes) | Allow local/remote toggle, ~1h effort | Native-only frontend (ocean) |
| 4 | CEO | Keep iframe approach | Taste | P5 (explicit) | Minimal rewrite, preserves web access | Tauri (high learning curve) |
| 5 | Design | Add startup progress steps | Mechanical | P1 (completeness) | Reduce user uncertainty during loading | - |
| 6 | Design | Add keyboard shortcuts | Mechanical | P1 (completeness) | Accessibility for desktop app | - |
| 7 | Eng | Add automated tests | Mechanical | P1 (completeness) | No test files in plan | Manual testing only |
| 8 | Eng | Add Windows code signing | Mechanical | P1 (completeness) | Unsigned triggers SmartScreen | - |
| 9 | DX | Add README for apps/desktop | Mechanical | P1 (completeness) | Missing standalone docs | - |

---

## Cross-Phase Themes

**Theme: Error handling** — flagged in CEO (Section 2), Design (Pass 2), Eng (Section 5), DX (Dimension 3). High-confidence signal. Must add visible error UI.

**Theme: Missing tests** — flagged in Eng only. Critical for desktop app reliability.

---

## NOT in Scope (Deferred to TODOS.md)

- System tray integration — not user-requested, defer
- Auto-update — marked optional in plan, defer until basic app works
- Native menus — not user-requested, defer
- Desktop notifications — not user-requested, defer
- macOS notarization — requires Apple Developer account, defer
- Splash screen — startup time acceptable, defer

---

## What Already Exists

| Sub-problem | Existing Code | Plan Reuses |
|-------------|---------------|-------------|
| Go backend daemon | `cmd/server/main.go` | Yes — bundled |
| React frontend | `web/src/App.tsx` | Yes — iframe embed |
| Daemon lifecycle pattern | multica daemon-manager.ts | Yes — adapted |
| IPC pattern | multica preload | Yes — similar |
| Build scripts | multica scripts | Yes — adapted |
| Packaging config | multica electron-builder.yml | Yes — similar |

---

## Review Scores Summary

- **CEO: 8.4/10** — Good strategy, 3 gaps identified
- **Design: 5.9/10** — Core states OK, error state and progress missing
- **Eng: 6.3/10** — Architecture good, tests critical gap
- **DX: 6.8/10** — Good naming, upgrade path missing

**Overall: 6.9/10** — Plan is executable but needs test plan and error handling before implementation.

---

Plan complete and saved to `docs/colink-desktop-app-2026-04-29.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints for review

**Which approach?**