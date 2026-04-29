import { describe, it, expect, vi, beforeEach } from "vitest";
import { getApiBaseUrl, getWsUrl } from "./api-bridge";

describe("api-bridge", () => {
  describe("getApiBaseUrl", () => {
    beforeEach(() => {
      vi.stubEnv("VITE_API_URL", "");
    });

    it("should return VITE_API_URL in remote mode", () => {
      vi.stubEnv("VITE_API_URL", "http://remote-server:26305");

      const result = getApiBaseUrl("remote");
      expect(result).toBe("http://remote-server:26305");
    });

    it("should fallback to localhost when VITE_API_URL is not set in remote mode", () => {
      vi.stubEnv("VITE_API_URL", "");

      const result = getApiBaseUrl("remote");
      expect(result).toBe("http://localhost:26305");
    });

    it("should return daemonUrl in local mode", () => {
      const result = getApiBaseUrl("local", "http://localhost:8080");
      expect(result).toBe("http://localhost:8080");
    });

    it("should fallback to default port in local mode when daemonUrl is not provided", () => {
      const result = getApiBaseUrl("local");
      expect(result).toBe("http://localhost:26305");
    });

    it("should handle various URL formats", () => {
      const urls = [
        "http://localhost:3000",
        "http://192.168.1.1:26305",
        "http://api.example.com",
        "https://secure-api.example.com:443",
      ];

      for (const url of urls) {
        const result = getApiBaseUrl("local", url);
        expect(result).toBe(url);
      }
    });
  });

  describe("getWsUrl", () => {
    beforeEach(() => {
      vi.stubEnv("VITE_API_URL", "");
    });

    it("should convert http to ws in remote mode", () => {
      vi.stubEnv("VITE_API_URL", "http://remote-server:26305");

      const result = getWsUrl("remote");
      expect(result).toBe("ws://remote-server:26305/ws");
    });

    it("should convert https to wss in remote mode", () => {
      vi.stubEnv("VITE_API_URL", "https://secure-server:26305");

      const result = getWsUrl("remote");
      expect(result).toBe("wss://secure-server:26305/ws");
    });

    it("should convert http to ws in local mode", () => {
      const result = getWsUrl("local", "http://localhost:26305");
      expect(result).toBe("ws://localhost:26305/ws");
    });

    it("should convert https to wss in local mode", () => {
      const result = getWsUrl("local", "https://localhost:26305");
      expect(result).toBe("wss://localhost:26305/ws");
    });

    it("should fallback to default WebSocket URL in local mode", () => {
      const result = getWsUrl("local");
      expect(result).toBe("ws://localhost:26305/ws");
    });

    it("should fallback to default WebSocket URL in remote mode when VITE_API_URL is not set", () => {
      vi.stubEnv("VITE_API_URL", "");

      const result = getWsUrl("remote");
      expect(result).toBe("ws://localhost:26305/ws");
    });

    it("should append /ws path to all URLs", () => {
      const testCases = [
        { mode: "local", daemonUrl: "http://localhost:3000", expected: "ws://localhost:3000/ws" },
        { mode: "local", daemonUrl: "http://192.168.1.1:26305", expected: "ws://192.168.1.1:26305/ws" },
        { mode: "remote", daemonUrl: undefined, expected: "ws://localhost:26305/ws" },
      ];

      for (const tc of testCases) {
        vi.stubEnv("VITE_API_URL", tc.mode === "remote" ? "" : "");
        const result = getWsUrl(tc.mode as "local" | "remote", tc.daemonUrl);
        expect(result).toBe(tc.expected);
      }
    });
  });
});