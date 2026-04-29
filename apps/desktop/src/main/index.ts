import { app, BrowserWindow, ipcMain, nativeImage } from "electron";
import { homedir } from "os";
import { join } from "path";
import { electronApp, optimizer, is } from "@electron-toolkit/utils";
import { setupDaemonManager } from "./daemon-manager";
import { openExternalSafely } from "./external-url";
import { installContextMenu } from "./context-menu";

const DEV_ICON_PATH = join(__dirname, "../../resources/icon.png");

// macOS/Linux GUI launches inherit minimal PATH from launchd.
// Run login shell to recover real PATH so bundled CLI can find agent binaries.
// Inline implementation to avoid ESM import issues in CommonJS bundle.
function fixPathForUnix(): void {
  if (process.platform === "win32") return;
  const fallbackPaths = [
    "/opt/homebrew/bin",
    "/usr/local/bin",
    join(homedir(), ".local/bin"),
  ];
  process.env.PATH = `${fallbackPaths.join(":")}:${process.env.PATH ?? ""}`;
}

fixPathForUnix();

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

// --- IPC handlers (register before window creation for preload sync calls) ---
ipcMain.on("app:get-info", (event) => {
  const p = process.platform;
  const os = p === "darwin" ? "macos" : p === "win32" ? "windows" : p === "linux" ? "linux" : "unknown";
  event.returnValue = { version: app.getVersion(), os, mode: targetApiBaseUrl ? "remote" : "local" };
});

ipcMain.handle("shell:openExternal", (_event, url: string) => {
  return openExternalSafely(url);
});

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
    const deepLinkUrl = argv.find((arg) => arg.startsWith(`${PROTOCOL}:`));
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

    // IPC: set target API URL (switch between local/remote)
    ipcMain.handle("daemon:set-target-api-url", async (_e, url: string) => {
      const normalized = url || null;
      if (targetApiBaseUrl !== normalized) {
        console.log(`[daemon] target API URL set to ${normalized ?? "(local)"}`);
        targetApiBaseUrl = normalized;
      }
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

  const deepLinkArg = process.argv.find((arg) => arg.startsWith(`${PROTOCOL}:`));
  if (deepLinkArg) {
    app.whenReady().then(() => handleDeepLink(deepLinkArg));
  }
}

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});