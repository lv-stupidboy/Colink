import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock Electron modules
vi.mock("electron", () => ({
  app: {
    getAppPath: vi.fn(() => "/mock/app/path"),
    getPath: vi.fn((name: string) => `/mock/${name}`),
    on: vi.fn(),
    quit: vi.fn(),
  },
  ipcMain: {
    handle: vi.fn(),
  },
  BrowserWindow: vi.fn(),
}));

// Mock child_process
vi.mock("child_process", () => ({
  execFile: vi.fn((_cmd, _args, _opts, cb) => {
    if (cb) cb(null);
    return undefined;
  }),
}));

// Mock fs
vi.mock("fs", () => ({
  existsSync: vi.fn(() => true),
}));

vi.mock("fs/promises", () => ({
  readFile: vi.fn(() => Promise.resolve(JSON.stringify({ autoStart: true, autoStop: false }))),
  writeFile: vi.fn(() => Promise.resolve()),
  mkdir: vi.fn(() => Promise.resolve()),
}));

// Import after mocks are set up
import { setupDaemonManager } from "../main/daemon-manager";
import { ipcMain } from "electron";

const mockIpcMain = ipcMain;

describe("daemon-manager", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("setupDaemonManager", () => {
    it("should register IPC handlers", () => {
      const mockWindowGetter = vi.fn(() => null);

      setupDaemonManager(mockWindowGetter, null);

      // Check IPC handlers are registered
      expect(mockIpcMain.handle).toHaveBeenCalledWith("daemon:start", expect.any(Function));
      expect(mockIpcMain.handle).toHaveBeenCalledWith("daemon:stop", expect.any(Function));
      expect(mockIpcMain.handle).toHaveBeenCalledWith("daemon:restart", expect.any(Function));
      expect(mockIpcMain.handle).toHaveBeenCalledWith("daemon:get-status", expect.any(Function));
      expect(mockIpcMain.handle).toHaveBeenCalledWith("daemon:get-prefs", expect.any(Function));
      expect(mockIpcMain.handle).toHaveBeenCalledWith("daemon:set-prefs", expect.any(Function));
      expect(mockIpcMain.handle).toHaveBeenCalledWith("daemon:auto-start", expect.any(Function));
      expect(mockIpcMain.handle).toHaveBeenCalledWith("daemon:set-target-url", expect.any(Function));
    });

    it("should send initial status via window", () => {
      const mockWindow = {
        webContents: {
          send: vi.fn(),
        },
      };
      const mockWindowGetter = vi.fn(() => mockWindow as unknown);

      setupDaemonManager(mockWindowGetter, null);

      // Initial status should be sent
      expect(mockWindow.webContents.send).toHaveBeenCalledWith(
        "daemon:status",
        expect.objectContaining({ state: expect.any(String) })
      );
    });

    it("should register IPC handlers in remote mode", () => {
      const remoteUrl = "http://remote-server:26305";
      const mockWindowGetter = vi.fn(() => null);

      setupDaemonManager(mockWindowGetter, remoteUrl);

      // Should still register IPC handlers even in remote mode
      expect(mockIpcMain.handle).toHaveBeenCalled();
    });
  });

  describe("module structure", () => {
    it("should export setupDaemonManager function", async () => {
      const module = await import("../main/daemon-manager");
      expect(typeof module.setupDaemonManager).toBe("function");
    });
  });
});