---
name: registry-version-error-handling
description: VERSION 文件缺失时改为报错而非使用默认值 1.0.0，同时修复 React useEffect 竞态条件
type: project
---

## 问题描述

installer-tauri setup 安装完成后，偶现注册表中 colink 版本号变成 1.0.0。

**根本原因分析**（两个层面）：

### 层面一：默认值 fallback 静默掩盖问题

VERSION 文件在某些环节缺失时，代码多处使用默认值 "1.0.0" 作为 fallback，问题被静默掩盖。

### 层面二：React useEffect 竞态条件

用户反馈"安装步骤的进度条加载两次"，分析发现：

**问题一：React.StrictMode 双重执行**

`src/main.tsx` 第 9 行使用 `<React.StrictMode>`，开发模式下会故意双重执行 useEffect 来测试副作用清理，导致进度条看起来加载两次。

**问题二：useEffect 依赖 version 导致重复注册**

`Installing.tsx` 第 252 行依赖数组 `[config, isRetrying, installType, oldInstallDir, version]`：

```
第一次 useEffect 执行：
  - version = null（初始值）
  - setupListener() 运行，事件监听器注册
  - 第 194-197 行：version 为 null → return（不启动安装）
  - setEventListenerReady(true) 触发渲染

version 更新后（异步）：
  - state 变化触发 useEffect cleanup
  - cleanup：isMounted=false, unlisten() 取消监听
  
第二次 useEffect 执行：
  - version = "1.2.5"
  - setupListener() 重新运行，事件监听器重新注册
  - version 有值 → 启动安装
```

**风险**：
- 每次 version 变化都会重新注册事件监听器
- 可能漏掉第一次监听期间的进度事件
- StrictMode + version 变化 = 监听器可能被注册/取消/重注册多次

## 当前行为

| 文件 | 行号 | 当前代码 | 问题 |
|------|------|----------|------|
| `src-tauri/src/services/installer.rs` | 782-791 | `if version_src.exists() { ... } else { warn!() }` | VERSION 不存在只 warn，继续安装 |
| `src-tauri/src/services/installer.rs` | 988 | `unwrap_or_else(|| "1.0.0")` | new_version 为 None 用默认值 |
| `src-tauri/src/commands/mode.rs` | 88-89 | `Ok("1.0.0".to_string())` | 找不到 VERSION 返回默认值 |
| `src/renderer/src/pages/Installing.tsx` | 110-121 | version 获取逻辑 | 失败时静默使用默认值（已部分修复） |
| `src/renderer/src/pages/Installing.tsx` | 252 | useEffect 依赖 version | version 变化导致监听器重复注册 |
| `src/main.tsx` | 9 | `<React.StrictMode>` | 开发模式双重执行 useEffect |

## 修改方案

### 1. installer.rs - VERSION 复制阶段报错

**修改位置**：第 782-791 行

```rust
// 当前代码（错误）
if version_src.exists() {
    std::fs::copy(&version_src, &version_dest)
        .map_err(|e| InstallerError::Io {
            context: "copy VERSION file".to_string(),
            source: e,
        })?;
    log::info!("Copied VERSION file to {:?}", version_dest);
} else {
    log::warn!("VERSION file not found at {:?}, using default version in registry", version_src);
}

// 修改为（正确）
if !version_src.exists() {
    return Err(InstallerError::Io {
        context: "VERSION file not found in resources".to_string(),
        source: std::io::Error::new(std::io::ErrorKind::NotFound, version_src.to_string_lossy()),
    });
}
std::fs::copy(&version_src, &version_dest)
    .map_err(|e| InstallerError::Io {
        context: "copy VERSION file".to_string(),
        source: e,
    })?;
log::info!("Copied VERSION file to {:?}", version_dest);
```

### 2. installer.rs - 注册表写入阶段报错

**修改位置**：第 988 行

```rust
// 当前代码（错误）
let version = config.new_version.clone().unwrap_or_else(|| "1.0.0".to_string());
write_registry(&config.install_dir, &version)?;

// 修改为（正确）
let version = config.new_version.clone()
    .ok_or_else(|| InstallerError::Config {
        context: "new_version not provided".to_string(),
    })?;
write_registry(&config.install_dir, &version)?;
```

### 3. mode.rs - getVersion API 报错

**修改位置**：第 88-89 行

```rust
// 当前代码（错误）
log::warn!("VERSION file not found, using default 1.0.0");
Ok("1.0.0".to_string()) // Default version

// 修改为（正确）
Err("VERSION file not found".to_string())
```

### 4. Installing.tsx - 修复 useEffect 竞态条件（核心修改）

**问题**：version 在依赖数组中，每次变化都会重新注册监听器。

**修改位置**：第 124-252 行的 useEffect

**方案：将 version 获取和安装启动分离为两个独立的 useEffect**

```typescript
// 第一个 useEffect：只获取 version（不依赖其他）
useEffect(() => {
  modeApi.getVersion().then(v => {
    console.log('[Installing] Got version:', v);
    setVersion(v);
  }).catch(e => {
    console.error('[Installing] Failed to get version:', e);
    setVersionError(e);
  });
}, []); // 空依赖数组，只在挂载时执行一次

// 第二个 useEffect：设置监听器并启动安装（不依赖 version）
useEffect(() => {
  let unlisten: (() => void) | undefined;
  let isMounted = true;

  // 只在 version 有值且无错误时启动安装
  if (versionError) {
    setInstallError('无法获取版本信息：' + versionError);
    setIsStarting(false);
    return;
  }
  if (!version) {
    // version 还在加载中，等待下一次 useEffect 触发
    // 但由于不依赖 version，这个 useEffect 只会在其他依赖变化时触发
    return;
  }

  const setupListener = async () => {
    // ... 设置监听器代码保持不变 ...
    
    // 启动安装（version 已有值）
    const installParams = {
      // ...
      newVersion: version, // 使用当前 version state
    };
    await installApi.startInstallation(installParams);
  };

  setupListener();

  return () => {
    isMounted = false;
    unlisten?.();
  };
}, [config, isRetrying, installType, oldInstallDir]); // 移除 version 依赖
```

**但这样有个问题**：移除 version 依赖后，version 从 null → 有值时不会触发 useEffect。

**更好方案：使用 ref 标记安装是否已启动**

```typescript
const installationStartedRef = useRef(false);

// version 获取 useEffect
useEffect(() => {
  modeApi.getVersion().then(v => {
    setVersion(v);
  }).catch(e => {
    setVersionError(e);
  });
}, []);

// 安装启动 useEffect - 当 version 有值时启动
useEffect(() => {
  // 条件：version 有值 + 未启动过 + 无错误
  if (!version || installationStartedRef.current || versionError) {
    return;
  }
  
  installationStartedRef.current = true; // 标记已启动，防止重复
  
  // 设置监听器并启动安装...
}, [version, versionError]); // 只依赖 version 相关
```

### 5. main.tsx - 移除 StrictMode（可选）

生产环境构建时 React.StrictMode 无影响，但开发模式会导致双重执行。可根据需要移除或保留。

```typescript
// 当前代码
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider locale={zhCN}>
      <App />
    </ConfigProvider>
  </React.StrictMode>
);

// 修改为（移除 StrictMode）
ReactDOM.createRoot(document.getElementById('root')!).render(
  <ConfigProvider locale={zhCN}>
    <App />
  </ConfigProvider>
);
```

## Why

用户报告：
1. 偶现版本号变成 1.0.0 - 静默 fallback 导致问题难以定位
2. 进度条加载两次 - useEffect 竞态条件 + StrictMode 双重执行

改为显式报错可暴露问题根源；修复 useEffect 竞态可消除重复执行。

## How to apply

1. 构建流程验证 VERSION 文件正确打包
2. 修改 Rust 代码将默认值改为报错
3. 修改 React useEffect 将 version 获取和安装启动分离
4. 使用 useRef 标记安装是否已启动，防止重复
5. 运行测试确保修改后安装流程正常报错且不重复执行

---

修复完成后：
- 偶现版本号问题变为必现报错，便于定位
- 进度条不会重复加载
- 开发模式和生产模式行为一致