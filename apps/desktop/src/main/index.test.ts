import { describe, it, expect, vi, beforeEach } from "vitest";

// Note: Testing Electron main process is complex due to module side effects.
// These tests focus on verifying the security fix: webSecurity should not be false.

describe("main/index security check", () => {
  describe("webPreferences security", () => {
    it("should verify webSecurity is not disabled in BrowserWindow creation", async () => {
      // Read the source file and verify the fix
      const fs = await import("fs/promises");
      const path = await import("path");

      const sourcePath = path.join(__dirname, "..", "main", "index.ts");
      const source = await fs.readFile(sourcePath, "utf-8");

      // Verify that webSecurity: false is NOT in the source
      expect(source).not.toMatch(/webSecurity:\s*false/);

      // Verify that sandbox: false is still there (needed for preload)
      expect(source).toMatch(/sandbox:\s*false/);
    });

    it("should verify preload path is correctly set", async () => {
      const fs = await import("fs/promises");
      const path = await import("path");

      const sourcePath = path.join(__dirname, "..", "main", "index.ts");
      const source = await fs.readFile(sourcePath, "utf-8");

      // Verify preload is configured
      expect(source).toMatch(/preload:\s*join\(__dirname,\s*["']\.\.\/preload\/index\.js["']\)/);
    });
  });
});