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