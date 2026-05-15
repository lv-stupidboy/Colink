# 注册表版本号默认值问题修复设计

## 问题描述

Setup 安装完后，偶先注册表中 colink 版本号变成 1.0.0，证明平常时是可以更改成功的，但在某些情况下会回退到默认值。

## 根源分析

### 版本号传递链路

```
VERSION 文件 → getPackageVersion() → newVersion → config.newVersion → writeRegistry()
```

### 三处 `'1.0.0'` 默认值

| 位置 | 文件 | 行号 | 代码 |
|------|------|------|------|
| 函数定义默认参数 | installer.ts | 1175 | `version: string = '1.0.0'` |
| 调用位置 fallback | installer.ts | 1468 | `config.newVersion || '1.0.0'` |
| getPackageVersion 返回默认值 | index.ts | 324, 335, 338 | `return ... || '1.0.0'` |

### 偶先变成 1.0.0 的原因

`getPackageVersion()` 函数在以下情况返回 `'1.0.0'`：

```typescript
function getPackageVersion(): string {
  try {
    // VERSION 文件不存在 → 跳过
    if (existsSync(versionPath)) {
      const content = require('fs').readFileSync(versionPath, 'utf-8')
      return content.trim() || '1.0.0'  // ⚠️ 空内容返回默认
    }

    // package.json fallback
    if (existsSync(packagePath)) {
      return pkg.version || '1.0.0'  // ⚠️ 无 version 返回默认
    }
  } catch {}
  return '1.0.0'  // ⚠️ 异常返回默认
}
```

**可能的触发场景**：

1. VERSION 文件不存在（打包漏拷贝或路径错误）
2. VERSION 文件为空（写入失败或被截断）
3. package.json 读取失败（app.asar 内路径问题）
4. 任意异常被 `catch {}` 吞掉

**最可能根本原因**：electron-builder 的 `extraResources` 配置 `to: "../packages/VERSION"` 在某些打包条件下未正确复制 VERSION 文件到 `packages/` 目录。

### 打包后 VERSION 文件路径

**运行时读取路径**：
```typescript
const versionPath = isDev
  ? join(__dirname, '../../VERSION')
  : join(process.resourcesPath, '../packages/VERSION')
```

**打包后目录结构**：
```
Colink Setup/
├── resources/
│   ├── app.asar (包含编译后的代码)
│   ├── icon.ico
│   ├── installer-config.json
├── packages/
│   ├── VERSION  ← 应该在这里
│   ├── desktop/
│   ├── runtime/
```

## 修复方案

### 方案 A：移除默认值，改为报错（用户推荐）

**修改内容**：

1. **移除 `writeRegistry` 的默认参数**
   ```typescript
   // 修改前
   export async function writeRegistry(installDir: string, version: string = '1.0.0')

   // 修改后
   export async function writeRegistry(installDir: string, version: string)
   ```

2. **移除调用位置的 fallback**
   ```typescript
   // 修改前
   await writeRegistry(config.installDir, config.newVersion || '1.0.0')

   // 修改后
   if (!config.newVersion) {
     throw new Error('无法获取安装版本号，请检查 VERSION 文件是否存在')
   }
   await writeRegistry(config.installDir, config.newVersion)
   ```

3. **修改 `getPackageVersion()` 返回错误而非默认值**
   ```typescript
   // 修改后
   function getPackageVersion(): string {
     // VERSION 文件优先
     const versionPath = isDev
       ? join(__dirname, '../../VERSION')
       : join(process.resourcesPath, '../packages/VERSION')

     if (existsSync(versionPath)) {
       const content = require('fs').readFileSync(versionPath, 'utf-8')
       const version = content.trim()
       if (!version) {
         throw new Error('VERSION 文件内容为空')
       }
       return version
     }

     // fallback: package.json
     const packagePath = isDev
       ? join(__dirname, '../../package.json')
       : join(process.resourcesPath, 'app.asar', 'package.json')

     if (existsSync(packagePath)) {
       const content = require('fs').readFileSync(packagePath, 'utf-8')
       const pkg = JSON.parse(content)
       if (!pkg.version) {
         throw new Error('package.json 中无 version 字段')
       }
       return pkg.version
     }

     throw new Error('无法获取版本号：VERSION 文件和 package.json 都不存在')
   }
   ```

4. **在 `start-installation` handler 中处理错误**
   ```typescript
   ipcMain.handle('start-installation', async (_event, config) => {
     try {
       const newVersion = getPackageVersion()
       // ...继续安装
     } catch (error) {
       // 向前端发送错误消息
       return { success: false, error: error.message }
     }
   })
   ```

**优点**：
- 明确报错，便于排查问题
- 不会写入错误的版本号
- 符合用户要求

**缺点**：
- 需要确保打包流程正确复制 VERSION 文件
- 安装流程可能因版本号读取失败而中断

### 方案 B：增加日志和检查（辅助排查）

在打包脚本中增加 VERSION 文件检查：

```powershell
# build.ps1
Copy-Item "$projectRoot\VERSION" "$installerDir\packages\VERSION" -Force
if (-not (Test-Path "$installerDir\packages\VERSION")) {
    Write-Host "ERROR: VERSION file not copied!" -ForegroundColor Red
    exit 1
}
$versionContent = Get-Content "$installerDir\packages\VERSION" -Raw
if ([string]::IsNullOrWhiteSpace($versionContent)) {
    Write-Host "ERROR: VERSION file is empty!" -ForegroundColor Red
    exit 1
}
Write-Host "VERSION file verified: $versionContent" -ForegroundColor Green
```

## 推荐方案

采用 **方案 A**（移除默认值，改为报错），同时结合 **方案 B** 的打包检查作为保障。

## 修改文件清单

| 文件 | 修改内容 |
|------|----------|
| installer/src/main/index.ts | getPackageVersion() 移除默认值返回，改为抛出错误 |
| installer/src/main/installer.ts | writeRegistry() 移除默认参数，调用处增加错误处理 |
| installer/build.ps1 | 增加 VERSION 文件复制后检查 |
| installer/build-fast.ps1 | 增加 VERSION 文件复制后检查 |

## 验证方案

1. 正常安装流程：VERSION 文件存在且内容正确 → 版本号正确写入注册表
2. VERSION 文件缺失：安装流程报错，提示用户检查 VERSION 文件
3. VERSION 文件为空：安装流程报错，提示用户检查 VERSION 文件
4. 打包流程验证：build.ps1/build-fast.ps1 检查 VERSION 文件复制成功