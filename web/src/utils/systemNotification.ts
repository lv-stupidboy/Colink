// web/src/utils/systemNotification.ts

/**
 * 系统通知工具
 * 使用 Web Notifications API 发送系统级通知
 * 即使浏览器最小化或切换到其他应用也能收到提醒
 */

// 通知权限状态
export type NotificationPermissionState = 'granted' | 'denied' | 'default' | 'unsupported';

// 通知配置
export interface SystemNotificationOptions {
  title: string;
  body: string;
  icon?: string;
  tag?: string; // 用于替换相同 tag 的通知
  requireInteraction?: boolean; // 是否需要用户交互才关闭
  onClick?: () => void;
}

// 存储权限请求的回调
let permissionRequestCallbacks: ((granted: boolean) => void)[] = [];

// 累积通知计数器
let pendingNotificationCount = 0;
let pendingAgentNames: string[] = [];
let lastNotification: Notification | null = null;

/**
 * 检查浏览器是否支持通知
 */
export function isNotificationSupported(): boolean {
  return 'Notification' in window;
}

/**
 * 获取当前通知权限状态
 */
export function getNotificationPermission(): NotificationPermissionState {
  if (!isNotificationSupported()) {
    return 'unsupported';
  }
  return Notification.permission as NotificationPermissionState;
}

/**
 * 检查是否已授权通知
 */
export function isNotificationGranted(): boolean {
  return getNotificationPermission() === 'granted';
}

/**
 * 请求通知权限
 * 返回 Promise，true 表示已授权
 */
export async function requestNotificationPermission(): Promise<boolean> {
  if (!isNotificationSupported()) {
    console.warn('[SystemNotification] 浏览器不支持通知 API');
    return false;
  }

  const currentPermission = getNotificationPermission();

  if (currentPermission === 'granted') {
    return true;
  }

  if (currentPermission === 'denied') {
    console.warn('[SystemNotification] 通知权限已被拒绝');
    return false;
  }

  // 请求权限
  try {
    const permission = await Notification.requestPermission();
    const granted = permission === 'granted';

    // 触发所有等待的回调
    permissionRequestCallbacks.forEach(cb => cb(granted));
    permissionRequestCallbacks = [];

    return granted;
  } catch (error) {
    console.error('[SystemNotification] 请求权限失败:', error);
    return false;
  }
}

/**
 * 清除累积通知计数
 */
export function clearPendingNotifications(): void {
  pendingNotificationCount = 0;
  pendingAgentNames = [];
  if (lastNotification) {
    lastNotification.close();
    lastNotification = null;
  }
  console.log('[SystemNotification] 累积通知已清除');
}

/**
 * 获取当前累积通知数量
 */
export function getPendingNotificationCount(): number {
  return pendingNotificationCount;
}

/**
 * 发送系统通知
 * @param options 通知配置
 * @returns 通知实例或 null（如果发送失败）
 */
export function sendSystemNotification(options: SystemNotificationOptions): Notification | null {
  if (!isNotificationGranted()) {
    console.warn('[SystemNotification] 未授权通知权限，尝试请求...');
    requestNotificationPermission().then(granted => {
      if (granted) {
        // 权限授权后重新发送
        sendSystemNotification(options);
      }
    });
    return null;
  }

  try {
    const notification = new Notification(options.title, {
      body: options.body,
      icon: options.icon || '/favicon.ico',
      tag: options.tag,
      requireInteraction: options.requireInteraction ?? false,
    });

    // 点击通知时的处理
    if (options.onClick) {
      notification.onclick = () => {
        options.onClick?.();
        notification.close();
        // 将焦点返回到窗口
        window.focus();
        // 清除累积计数
        clearPendingNotifications();
      };
    } else {
      // 默认行为：点击时聚焦窗口并清除累积
      notification.onclick = () => {
        notification.close();
        window.focus();
        clearPendingNotifications();
      };
    }

    console.log('[SystemNotification] 通知已发送:', options.title);
    return notification;
  } catch (error) {
    console.error('[SystemNotification] 发送通知失败:', error);
    return null;
  }
}

/**
 * 发送 Agent 完成通知（累积模式）
 * 每次调用累积计数，更新通知内容显示总数量
 */
export function sendAgentCompletionNotification(agentName: string, summary?: string): Notification | null {
  // 累积计数
  pendingNotificationCount++;
  pendingAgentNames.push(agentName);

  // 关闭旧通知（准备发送新的累积通知）
  if (lastNotification) {
    lastNotification.close();
  }

  // 构建通知内容
  const title = pendingNotificationCount === 1
    ? 'Agent 执行完成'
    : `${pendingNotificationCount} 个 Agent 已完成`;

  let body: string;
  if (pendingNotificationCount === 1) {
    body = summary || `${agentName} 已完成执行，请查看结果`;
  } else {
    // 多个 Agent：显示列表（最多显示前3个）
    const displayNames = pendingAgentNames.slice(-3);
    body = `已完成：${displayNames.join('、')}`;
    if (pendingAgentNames.length > 3) {
      body += ` 等 ${pendingNotificationCount} 个 Agent`;
    }
  }

  console.log('[SystemNotification] 累积通知:', {
    count: pendingNotificationCount,
    agents: pendingAgentNames,
    title,
    body
  });

  // 发送新的累积通知
  lastNotification = sendSystemNotification({
    title,
    body,
    tag: 'isdp-agent-complete', // 使用固定 tag，替换旧通知但内容更新
    requireInteraction: true, // Agent 完成通知需要用户点击确认
  });

  return lastNotification;
}

/**
 * 检查并显示权限提示（如果未授权）
 * 在用户首次触发 Agent 时调用
 */
export function checkAndPromptPermission(): boolean {
  const permission = getNotificationPermission();

  if (permission === 'unsupported') {
    console.warn('[SystemNotification] 浏览器不支持系统通知');
    return false;
  }

  if (permission === 'denied') {
    console.warn('[SystemNotification] 用户已拒绝通知权限');
    return false;
  }

  if (permission === 'default') {
    // 权限未请求，显示友好提示
    console.log('[SystemNotification] 需要请求通知权限');
    return false;
  }

  return true;
}

// 导出默认对象
const SystemNotification = {
  isSupported: isNotificationSupported,
  getPermission: getNotificationPermission,
  isGranted: isNotificationGranted,
  requestPermission: requestNotificationPermission,
  send: sendSystemNotification,
  sendAgentCompletion: sendAgentCompletionNotification,
  checkAndPrompt: checkAndPromptPermission,
  clearPending: clearPendingNotifications,
  getPendingCount: getPendingNotificationCount,
};

export default SystemNotification;