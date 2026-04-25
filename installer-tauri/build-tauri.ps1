#!/usr/bin/env pwsh
# Build script for Colink Installer (Tauri)
# Windows PowerShell version

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$InstallerDir = $PSScriptRoot
$DistDir = "$InstallerDir/dist"
$ReleaseDir = "$InstallerDir/src-tauri/target/release"

Write-Host "=== Colink Installer (Tauri) Build ===" -ForegroundColor Cyan

# Step 1: Check prerequisites
Write-Host "Checking prerequisites..." -ForegroundColor Yellow

# Check Rust
try {
    $rustVersion = rustc --version
    Write-Host "Rust: $rustVersion" -ForegroundColor Green
} catch {
    Write-Host "ERROR: Rust not installed. Install from https://rustup.rs" -ForegroundColor Red
    exit 1
}

# Check Node.js
try {
    $nodeVersion = node --version
    Write-Host "Node.js: $nodeVersion" -ForegroundColor Green
} catch {
    Write-Host "ERROR: Node.js not installed" -ForegroundColor Red
    exit 1
}

# Check pnpm
try {
    $pnpmVersion = pnpm --version
    Write-Host "pnpm: $pnpmVersion" -ForegroundColor Green
} catch {
    Write-Host "WARNING: pnpm not installed, using npm" -ForegroundColor Yellow
}

# Step 2: Install dependencies
Write-Host "Installing frontend dependencies..." -ForegroundColor Yellow
Set-Location $InstallerDir

if (Get-Command pnpm -ErrorAction SilentlyContinue) {
    pnpm install
} else {
    npm install
}

# Step 3: Build frontend
Write-Host "Building frontend..." -ForegroundColor Yellow
if (Get-Command pnpm -ErrorAction SilentlyContinue) {
    pnpm run build:renderer
} else {
    npm run build:renderer
}

# Step 4: Build Tauri
Write-Host "Building Tauri application..." -ForegroundColor Yellow
if (Get-Command pnpm -ErrorAction SilentlyContinue) {
    pnpm run build
} else {
    npm run build
}

# Step 5: Create Launcher exe (copy and rename)
Write-Host "Creating Launcher exe..." -ForegroundColor Yellow
$SetupExe = "$ReleaseDir/colink-installer.exe"
$LauncherExe = "$ReleaseDir/Colink.exe"

if (Test-Path $SetupExe) {
    Copy-Item $SetupExe $LauncherExe -Force
    Write-Host "Created: $LauncherExe" -ForegroundColor Green
} else {
    Write-Host "WARNING: Setup exe not found at $SetupExe" -ForegroundColor Yellow
}

# Step 6: Create output directory
Write-Host "Organizing output..." -ForegroundColor Yellow
$OutputDir = "$InstallerDir/output"
if (Test-Path $OutputDir) {
    Remove-Item $OutputDir -Recurse -Force
}
New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null

# Copy NSIS installer
$NsisFiles = Get-ChildItem "$ReleaseDir/*.exe" -Exclude "colink-installer.exe"
foreach ($file in $NsisFiles) {
    Copy-Item $file.FullName $OutputDir -Force
}

# Copy Launcher exe
if (Test-Path $LauncherExe) {
    Copy-Item $LauncherExe $OutputDir -Force
}

Write-Host "=== Build Complete ===" -ForegroundColor Cyan
Write-Host "Output directory: $OutputDir" -ForegroundColor Green
Get-ChildItem $OutputDir | Format-Table Name, Length -AutoSize