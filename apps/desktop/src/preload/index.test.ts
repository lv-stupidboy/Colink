import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { DaemonPrefs } from "../shared/daemon-types";

type DaemonAPIType = {
  start: () => Promise<{ success: boolean }>;
  stop: () => Promise<{ success: boolean }>;
  restart: () => Promise<{ success: boolean }>;
  getStatus: () => Promise<{ state: string }>;
  getPrefs: () => Promise<DaemonPrefs>;
  setPrefs: (prefs: Partial<DaemonPrefs>) => Promise<Partial<DaemonPrefs>>;
  autoStart: () => Promise<void>;
  setTargetUrl: (url: string | null) => Promise<void>;
  onStatusChange: (cb: (status: unknown) => void) => () => void;
};

type DesktopAPIType = {
  appInfo: { version: string; os: string; mode: string };
  openExternal: (url: string) => Promise<void>;
  onAuthToken: (cb: (token: string) => void) => () => void;
};

// Mock Electron modules before import
vi.mock("electron", () => ({
  contextBridge: {
    exposeInMainWorld: vi.fn((key: string, api: unknown) => {
      (globalThis as Record<string, unknown>)[key] = api;
    }),
  },
  ipcRenderer: {
    invoke: vi.fn(async (channel: string, ...args: unknown[]) => {
      if (channel === "daemon:start") return { success: true };
      if (channel === "daemon:stop") return { success: true };
      if (channel === "daemon:restart") return { success: true };
      if (channel === "daemon:get-status") return { state: "stopped" };
      if (channel === "daemon:get-prefs") return { autoStart: true, autoStop: false };
      if (channel === "daemon:set-prefs") return args[0];
      if (channel === "daemon:auto-start") return undefined;
      if (channel === "daemon:set-target-url") return undefined;
      if (channel === "shell:openExternal") return undefined;
      return undefined;
    }),
    on: vi.fn(),
    removeListener: vi.fn(),
    sendSync: vi.fn(() => ({ version: "1.0.0", os: "windows", mode: "local" })),
  },
}));

describe("preload", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    delete (globalThis as Record<string, unknown>).daemonAPI;
    delete (globalThis as Record<string, unknown>).desktopAPI;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("daemonAPI", () => {
    it("should expose daemon API functions", async () => {
      vi.resetModules();
      await import("../preload/index");

      const daemonAPI = (globalThis as Record<string, unknown>).daemonAPI as DaemonAPIType;
      expect(daemonAPI).toBeDefined();
      expect(typeof daemonAPI.start).toBe("function");
      expect(typeof daemonAPI.stop).toBe("function");
      expect(typeof daemonAPI.restart).toBe("function");
      expect(typeof daemonAPI.getStatus).toBe("function");
      expect(typeof daemonAPI.getPrefs).toBe("function");
      expect(typeof daemonAPI.setPrefs).toBe("function");
      expect(typeof daemonAPI.autoStart).toBe("function");
      expect(typeof daemonAPI.setTargetUrl).toBe("function");
      expect(typeof daemonAPI.onStatusChange).toBe("function");
    });

    it("should invoke daemon:start when start() called", async () => {
      vi.resetModules();
      await import("../preload/index");

      const daemonAPI = (globalThis as Record<string, unknown>).daemonAPI as DaemonAPIType;
      const result = await daemonAPI.start();

      expect(result).toEqual({ success: true });
    });

    it("should invoke daemon:stop when stop() called", async () => {
      vi.resetModules();
      await import("../preload/index");

      const daemonAPI = (globalThis as Record<string, unknown>).daemonAPI as DaemonAPIType;
      const result = await daemonAPI.stop();

      expect(result).toEqual({ success: true });
    });

    it("should invoke daemon:get-status when getStatus() called", async () => {
      vi.resetModules();
      await import("../preload/index");

      const daemonAPI = (globalThis as Record<string, unknown>).daemonAPI as DaemonAPIType;
      const result = await daemonAPI.getStatus();

      expect(result).toEqual({ state: "stopped" });
    });

    it("should invoke daemon:set-prefs with prefs object", async () => {
      vi.resetModules();
      await import("../preload/index");

      const daemonAPI = (globalThis as Record<string, unknown>).daemonAPI as DaemonAPIType;
      const prefs = { autoStart: false };
      const result = await daemonAPI.setPrefs(prefs);

      expect(result).toEqual(prefs);
    });

    it("should register and remove status change listener", async () => {
      vi.resetModules();
      await import("../preload/index");

      const daemonAPI = (globalThis as Record<string, unknown>).daemonAPI as DaemonAPIType;
      const callback = vi.fn();
      const cleanup = daemonAPI.onStatusChange(callback);

      expect(typeof cleanup).toBe("function");
      cleanup();

      const { ipcRenderer } = await import("electron");
      expect(ipcRenderer.removeListener).toHaveBeenCalled();
    });
  });

  describe("desktopAPI", () => {
    it("should expose desktop API functions", async () => {
      vi.resetModules();
      await import("../preload/index");

      const desktopAPI = (globalThis as Record<string, unknown>).desktopAPI as DesktopAPIType;
      expect(desktopAPI).toBeDefined();
      expect(typeof desktopAPI.openExternal).toBe("function");
      expect(typeof desktopAPI.onAuthToken).toBe("function");
      expect(desktopAPI.appInfo).toBeDefined();
    });

    it("should have appInfo from sendSync", async () => {
      vi.resetModules();
      await import("../preload/index");

      const desktopAPI = (globalThis as Record<string, unknown>).desktopAPI as DesktopAPIType;
      expect(desktopAPI.appInfo).toEqual({ version: "1.0.0", os: "windows", mode: "local" });
    });

    it("should register and remove auth token listener", async () => {
      vi.resetModules();
      await import("../preload/index");

      const desktopAPI = (globalThis as Record<string, unknown>).desktopAPI as DesktopAPIType;
      const callback = vi.fn();
      const cleanup = desktopAPI.onAuthToken(callback);

      expect(typeof cleanup).toBe("function");
      cleanup();

      const { ipcRenderer } = await import("electron");
      expect(ipcRenderer.removeListener).toHaveBeenCalled();
    });
  });
});