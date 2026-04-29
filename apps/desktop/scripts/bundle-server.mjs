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