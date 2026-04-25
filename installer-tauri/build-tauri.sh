#!/bin/bash
# Build script for Colink Installer (Tauri)
# Unix/Linux/macOS version

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER_DIR="$SCRIPT_DIR"

echo "=== Colink Installer (Tauri) Build ==="

# Step 1: Check prerequisites
echo "Checking prerequisites..."

# Check Rust
if ! command -v rustc &> /dev/null; then
    echo "ERROR: Rust not installed. Install from https://rustup.rs"
    exit 1
fi
echo "Rust: $(rustc --version)"

# Check Node.js
if ! command -v node &> /dev/null; then
    echo "ERROR: Node.js not installed"
    exit 1
fi
echo "Node.js: $(node --version)"

# Check pnpm
if command -v pnpm &> /dev/null; then
    echo "pnpm: $(pnpm --version)"
    PM="pnpm"
else
    echo "WARNING: pnpm not installed, using npm"
    PM="npm"
fi

# Step 2: Install dependencies
echo "Installing frontend dependencies..."
cd "$INSTALLER_DIR"
$PM install

# Step 3: Build frontend
echo "Building frontend..."
$PM run build:renderer

# Step 4: Build Tauri
echo "Building Tauri application..."
$PM run build

# Step 5: Create Launcher exe (copy and rename)
echo "Creating Launcher exe..."
RELEASE_DIR="$INSTALLER_DIR/src-tauri/target/release"
SETUP_EXE="$RELEASE_DIR/colink-installer"
LAUNCHER_EXE="$RELEASE_DIR/Colink"

if [ -f "$SETUP_EXE" ]; then
    cp "$SETUP_EXE" "$LAUNCHER_EXE"
    echo "Created: $LAUNCHER_EXE"
fi

# Step 6: Create output directory
echo "Organizing output..."
OUTPUT_DIR="$INSTALLER_DIR/output"
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Copy all build artifacts
cp -r "$RELEASE_DIR"/bundle/* "$OUTPUT_DIR/" 2>/dev/null || true
if [ -f "$LAUNCHER_EXE" ]; then
    cp "$LAUNCHER_EXE" "$OUTPUT_DIR/"
fi

echo "=== Build Complete ==="
echo "Output directory: $OUTPUT_DIR"
ls -la "$OUTPUT_DIR"