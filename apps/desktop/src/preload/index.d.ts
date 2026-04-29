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