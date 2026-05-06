---
title: installer-tauri Mac 支持代码审查报告
date: 2026-05-06
version: 1.0.0
status: completed
---

# installer-tauri Mac 支持代码审查报告

## 审查范围

- 新增文件：`bundle.rs`, `plist.rs`
- 改造文件：`installer.rs`, `service_manager.rs`, `file_ops.rs`, `shortcut.rs`, `registry.rs`, `uninstall.rs`, `config.rs`, `mod.rs`

## CRITICAL 问题修复验证

### CRITICAL-01：数据存储位置 ✅ 已正确修复

| 文件 | 行号 | 验证内容 | 状态 |
|------|------|----------|------|
| `bundle.rs` | 7-11 | `get_mac_data_dir()` 返回 `~/Library/Application Support/Colink` | ✅ |
| `bundle.rs` | 84-86 | 注释明确说明不在 App Bundle 内创建 data 目录 | ✅ |
| `plist.rs` | 26-36 | 数据目录路径正确使用 `dirs::data_dir()` | ✅ |
| `plist.rs` | 52-53 | has_data 检测指向数据目录的数据库文件 | ✅ |
| `config.rs` | 127-145 | Mac 配置文件读写路径正确 | ✅ |
| `service_manager.rs` | 162-168 | 服务启动配置路径指向数据目录 | ✅ |
| `service_manager.rs` | 211-267 | 首次运行自动初始化数据目录逻辑完整 | ✅ |

**结论**：数据目录已正确移至 `~/Library/Application Support/Colink/`，升级时不会丢失。

### CRITICAL-02：.exe 硬编码 ✅ 已正确修复

| 文件 | 行号 | 设计标注 | 实际代码 | 状态 |
|------|------|----------|----------|------|
| `service_manager.rs` | 148-153 | server_exe 路径 | `#[cfg]` 分支，Mac 使用 `Contents/MacOS/colink-server` | ✅ |
| `installer.rs` | 197-206 | migrate.exe 路径 | `#[cfg]` 分支，Mac 无 .exe 后缀 | ✅ |
| `installer.rs` | 286-295 | migrate.exe 路径 | `#[cfg]` 分支 | ✅ |
| `installer.rs` | 621-636 | server_src 路径 | 兼容 Windows/Mac 构建产物 | ✅ |
| `installer.rs` | 712-738 | Launcher 路径 | 多候选路径列表（含 .exe 和无后缀） | ✅ |
| `uninstall.rs` | 17-27 | 进程名 | `#[cfg]` 分支，Mac 无 .exe | ✅ |
| `uninstall.rs` | 65-84 | 进程检测 | `#[cfg]` 分支 | ✅ |
| `uninstall.rs` | 94-105 | 进程终止 | `#[cfg]` 分支 | ✅ |
| `file_ops.rs` | 418-434 | 进程检测 | `pgrep -x` 实现 | ✅ |
| `file_ops.rs` | 461-477 | 进程终止 | `pkill -x` 实现 | ✅ |

**结论**：所有 `.exe` 硬编码已通过 `#[cfg(target_os)]` 条件编译修复。

## 其他模块验证

### registry.rs（Mac plist 分支）

| 行号 | 验证内容 | 状态 |
|------|----------|------|
| 78-93 | `get_installed_version()` Mac 分支调用 plist 函数 | ✅ |
| 174-187 | `write_registry()` Mac 分支调用 plist 函数 | ✅ |
| 203-210 | `delete_registry()` Mac 分支调用 plist 函数 | ✅ |

### shortcut.rs（Mac 空实现）

| 行号 | 验证内容 | 状态 |
|------|----------|------|
| 24-29 | `create_desktop_shortcut()` Mac 返回 Ok | ✅ |
| 50-55 | `create_start_menu_shortcut()` Mac 返回 Ok | ✅ |
| 155-160 | `delete_desktop_shortcut()` Mac 返回 Ok | ✅ |
| 184-188 | `delete_start_menu_shortcut()` Mac 返回 Ok | ✅ |

**结论**：符合设计（Mac App Bundle 自动出现在 Launchpad，无需手动创建快捷方式）。

## 代码质量评估

### 优点

1. **平台隔离正确**：所有平台差异使用 `#[cfg(target_os)]` 条件编译
2. **错误处理统一**：统一使用 `InstallerError` 类型
3. **路径处理完整**：兼容 Windows 和 Mac 构建产物
4. **首次运行逻辑完整**：Mac 首次运行自动创建数据目录并初始化配置
5. **plist 使用规范**：按审查建议使用 plist crate 解析而非 regex

### 待验证项

| 项目 | 状态 | 备注 |
|------|------|------|
| Windows 编译 | ✅ 通过 | 全栈开发工程师已验证 |
| macOS 编译 | ⏳ 待验证 | 需在 Mac 环境编译 |
| plist crate 依赖 | ✅ 已添加 | `Cargo.toml` 中有配置 |
| 非目标平台 stub | ✅ 正确 | `#[cfg(not(target_os = "macos"))]` 有 stub 函数 |

## 审查结论

### 通过项

1. ✅ CRITICAL-01（数据存储位置）正确修复
2. ✅ CRITICAL-02（.exe 硬编码）正确修复
3. ✅ 所有设计标注的修改点已实现
4. ✅ 代码质量良好，符合跨平台最佳实践

### 建议后续测试

1. 在 Mac 环境编译验证（`cargo build --target aarch64-apple-darwin`）
2. 运行 Mac 单元测试验证数据目录位置
3. 模拟升级场景验证数据保留

## 测试建议

### 必要测试（Mac 环境）

```bash
# 编译测试
cd installer-tauri/src-tauri
cargo build --release --target aarch64-apple-darwin

# 数据目录测试
# 验证 get_mac_data_dir() 返回 ~/Library/Application Support/Colink
# 验证首次运行自动创建数据目录
```

### 升级场景测试

1. 创建旧版本 App Bundle + 数据目录
2. 覆盖 App Bundle
3. 验证数据目录内容保留

---

## A2A 交接信息

### What
installer-tauri Mac 支持代码审查完成，CRITICAL-01 和 CRITICAL-02 正确修复，Windows 编译通过。

### Why
全栈开发工程师完成 Mac 支持改造后，需要进行代码审查验证修复正确性。

### Next
无需下游：代码审查通过，建议在 Mac 环境进行编译测试验证。