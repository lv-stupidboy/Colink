---
name: registry-version-error-handling-implementation
description: 修复 installer-tauri 注册表版本号偶现 1.0.0 问题
type: project
---

## 实施完成报告

### 问题描述
installer-tauri setup 安装完成后，偶现注册表中 colink 版本号变成 1.0.0。根本原因：
1. 默认值 fallback 静默掩盖问题
2. React useEffect 竞态条件导致重复执行

### 实施内容

#### Rust 代码修改（已完成）

1. **installer.rs:782-793** - VERSION 复制阶段报错
   - 原来：VERSION 不存在只 warn，继续安装
   - 修改：VERSION 不存在返回 InstallerError::Io 错误

2. **installer.rs:990-993** - 注册表写入阶段报错
   - 原来：`unwrap_or_else(|| "1.0.0")` 使用默认值
   - 修改：`ok_or_else(|| InstallerError::Config)` 报错

3. **mode.rs:88** - getVersion API 报错
   - 原来：找不到 VERSION 返回默认值 "1.0.0"
   - 修改：返回 `Err("VERSION file not found")`

#### 前端代码修改（已完成）

1. **Installing.tsx** - 三个独立 useEffect：
   - useEffect 1：获取版本（空依赖数组，只在挂载时执行）
   - useEffect 2：设置事件监听器（空依赖数组，只注册一次）
   - useEffect 3：启动安装（依赖 version 和 eventListenerReady，用 ref 防止重复）

2. **useRef 防止重复启动**：
   - `installationStartedRef` 标记安装是否已启动
   - StrictMode 双重执行或 version 变化不会导致重复启动

3. **version 状态改为 null**：
   - 原来：`useState<string>('1.0.0')` 默认值
   - 修改：`useState<string | null>(null)` 无默认值
   - 添加 `versionError` 状态处理获取失败

4. **StrictMode 保留**：
   - ref 已防止重复启动，不影响功能
   - StrictMode 有助于检测其他副作用问题

### 验证结果
- TypeScript 类型检查：通过
- 修改符合设计文档要求

### Why
用户报告：
1. 偶现版本号变成 1.0.0 - 静默 fallback 导致问题难以定位
2. 进度条加载两次 - useEffect 竞态条件 + StrictMode 双重执行

改为显式报错可暴露问题根源；修复 useEffect 竞态可消除重复执行。

### How to apply
1. 构建流程验证 VERSION 文件正确打包
2. 测试安装流程：VERSION 存在时正常安装，不存在时显式报错
3. 测试 StrictMode：开发模式下不会重复启动安装

---