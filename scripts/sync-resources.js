#!/usr/bin/env node
/**
 * 同步资源从主项目到指定目录
 * 用法: node scripts/sync-resources.js [目标目录]
 * 默认目标: installer-tauri/src-tauri/target/release/staging/resources
 */

const fs = require('fs');
const path = require('path');

const ROOT_DIR = path.resolve(__dirname, '..');

// 目标目录：默认为 staging 目录，或传入的目录
const TARGET_DIR = process.argv[2] || path.join(ROOT_DIR, 'installer-tauri', 'src-tauri', 'target', 'release', 'staging', 'resources');

console.log('=== 同步资源 ===');
console.log('主项目目录:', ROOT_DIR);
console.log('目标目录:', TARGET_DIR);

// 确保目标目录存在
if (!fs.existsSync(TARGET_DIR)) {
  fs.mkdirSync(TARGET_DIR, { recursive: true });
}

// 1. 复制 colink-server.exe
const serverSrc = path.join(ROOT_DIR, 'bin', 'colink-server.exe');
const serverDest = path.join(TARGET_DIR, 'colink-server.exe');
if (fs.existsSync(serverSrc)) {
  fs.copyFileSync(serverSrc, serverDest);
  console.log('✓ colink-server.exe');
} else {
  console.error('✗ colink-server.exe 未找到，请先执行 make build');
}

// 2. 复制 migrate.exe 到 bin 目录
const binDestDir = path.join(TARGET_DIR, 'bin');
if (!fs.existsSync(binDestDir)) {
  fs.mkdirSync(binDestDir, { recursive: true });
}
const migrateSrc = path.join(ROOT_DIR, 'bin', 'migrate.exe');
const migrateDest = path.join(binDestDir, 'migrate.exe');
if (fs.existsSync(migrateSrc)) {
  fs.copyFileSync(migrateSrc, migrateDest);
  console.log('✓ migrate.exe');
} else {
  console.error('✗ migrate.exe 未找到');
}

// 2.5 复制 mcp-server 到 packages/runtime/tools 目录（提供 post_message 等 A2A 工具）
const isWindows = process.platform === 'win32';
const mcpServerName = isWindows ? 'mcp-server.exe' : 'mcp-server';
const toolsDestDir = path.join(TARGET_DIR, 'packages', 'runtime', 'tools');
if (!fs.existsSync(toolsDestDir)) {
  fs.mkdirSync(toolsDestDir, { recursive: true });
}
const mcpServerSrc = path.join(ROOT_DIR, 'bin', mcpServerName);
const mcpServerDest = path.join(toolsDestDir, mcpServerName);
if (fs.existsSync(mcpServerSrc)) {
  fs.copyFileSync(mcpServerSrc, mcpServerDest);
  console.log('✓ ' + mcpServerName);
} else {
  console.error('✗ ' + mcpServerName + ' 未找到，请先执行 make build');
}

// 3. 复制 sql-change 目录
const sqlSrc = path.join(ROOT_DIR, 'sql-change');
const sqlDest = path.join(TARGET_DIR, 'sql-change');
if (fs.existsSync(sqlSrc)) {
  if (fs.existsSync(sqlDest)) {
    fs.rmSync(sqlDest, { recursive: true });
  }
  copyDir(sqlSrc, sqlDest);
  console.log('✓ sql-change/');
} else {
  console.error('✗ sql-change/ 未找到');
}

// 4. 复制 web/dist 目录
const webSrc = path.join(ROOT_DIR, 'web', 'dist');
const webDest = path.join(TARGET_DIR, 'web');
if (fs.existsSync(webSrc)) {
  if (fs.existsSync(webDest)) {
    fs.rmSync(webDest, { recursive: true });
  }
  copyDir(webSrc, webDest);
  console.log('✓ web/');
} else {
  console.error('✗ web/dist 未找到，请先执行 cd web && npm run build');
}

// 5. 复制 config.yaml.example
const configSrc = path.join(ROOT_DIR, 'configs', 'config.yaml.example');
const configDest = path.join(TARGET_DIR, 'config.yaml.example');
if (fs.existsSync(configSrc)) {
  fs.copyFileSync(configSrc, configDest);
  console.log('✓ config.yaml.example');
} else {
  console.error('✗ config.yaml.example 未找到');
}

console.log('\n=== 同步完成 ===');

// 辅助函数：递归复制目录
function copyDir(src, dest) {
  fs.mkdirSync(dest, { recursive: true });
  const entries = fs.readdirSync(src, { withFileTypes: true });

  for (const entry of entries) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);

    if (entry.isDirectory()) {
      copyDir(srcPath, destPath);
    } else {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}