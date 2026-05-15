# Launcher 关闭时服务状态检查设计

## 需求

关闭 Tauri Launcher 应用时，检查 colink-server 是否正在运行。若运行中，提示用户选择停止服务或取消关闭。

## 背景

旧 Electron Launcher 已实现此功能（`installer/src/main/shared/window-utils.ts`），新 Tauri Launcher 缺失该检查，可能导致服务意外残留。

## 设计方案

### 交互流程

```
用户点击关闭按钮
    ↓
检查 ServiceManager.is_running()
    ↓
┌─ 运行中 ──────────────────────┐
│  弹窗提示                      │
│  - "停止服务并关闭" → stop() → 关闭 │
│  - "取消" → 阻止关闭           │
└───────────────────────────────┘
┌─ 未运行 ──────────────────────┐
│  直接关闭                      │
└───────────────────────────────┘
```

### 技术架构

**新增 Rust 命令** (`src-tauri/src/commands/window.rs`):

```rust
#[derive(serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub enum CloseResult {
    AllowClose,
    BlockClose,
}

#[tauri::command]
pub async fn window_close_with_confirm(
    app: AppHandle,
    state: State<'_, AppState>
) -> Result<CloseResult, String> {
    // 1. 检查服务状态
    // 2. 运行中则弹窗确认
    // 3. 返回关闭决策
}
```

**依赖现有方法**:
- `ServiceManager::is_running()` - 检查服务状态
- `ServiceManager::stop()` - 停止服务

**前端修改** (`src/lib/api/window.ts`):

```typescript
export async function windowClose(): Promise<{ result: 'allowClose' | 'blockClose' }> {
  return invoke('window_close_with_confirm')
}
```

### 修改清单

| 文件 | 修改 |
|------|------|
| `commands/window.rs` | 新增 `window_close_with_confirm` 命令 |
| `commands/mod.rs` | 导出新命令 |
| `lib.rs` | 注册新命令到 Tauri |
| `src/lib/api/window.ts` | 更新 `windowClose()` API |
| `src/renderer/src/components/Layout.tsx` | 处理 `blockClose` 返回值 |

### 错误处理

- 服务停止失败：弹窗提示错误，阻止关闭
- 对话框 API 失败：日志记录，默认阻止关闭

### 测试要点

1. 服务运行时关闭 → 弹窗出现 → 选择取消 → 窗口保持打开
2. 服务运行时关闭 → 弹窗出现 → 选择停止 → 服务停止 → 窗口关闭
3. 服务停止时关闭 → 直接关闭，无弹窗
4. 服务停止失败 → 弹窗报错，窗口保持打开

## 参考实现

旧 Electron Launcher (`installer/src/main/shared/window-utils.ts`):
- `showCloseConfirm()` 函数
- `dialog.showMessageBox()` 弹窗
- `checkServiceRunning()` / `stopService()` 回调