// isdp/web/src/config/agentStyles.ts
import type { AgentRole } from '@/types';

/**
 * Agent 样式配置
 * 基于 Agent 角色类型定义不同的气泡样式
 */

export interface AgentStyleConfig {
  radius: string;      // 气泡圆角样式
  font?: string;       // 字体样式（可选）
  color: string;       // 主色调（用于 @提及高亮）
}

/**
 * Agent 角色样式映射
 * 不同角色有不同的圆角样式，统一白色背景
 */
export const AGENT_STYLES: Record<AgentRole, AgentStyleConfig> = {
  requirement: {
    radius: 'rounded-2xl rounded-bl-sm',
    color: '#1890ff',
  },
  architect: {
    radius: 'rounded-2xl rounded-br-sm',
    font: 'font-mono',
    color: '#722ed1',
  },
  developer: {
    radius: 'rounded-2xl rounded-tr-sm',
    color: '#52c41a',
  },
  reviewer: {
    radius: 'rounded-lg rounded-tl-sm',
    font: 'font-mono',
    color: '#faad14',
  },
  testengineer: {
    radius: 'rounded-2xl rounded-bl-md',
    color: '#eb2f96',
  },
  devops: {
    radius: 'rounded-xl rounded-tr-md',
    font: 'font-mono',
    color: '#13c2c2',
  },
  fullstack_engineer: {
    radius: 'rounded-2xl rounded-br-md',
    color: '#2f54eb',
  },
  custom: {
    radius: 'rounded-xl',
    color: '#595959',
  },
};

/**
 * 获取 Agent 样式
 * @param role Agent 角色类型
 * @returns Agent 样式配置
 */
export function getAgentStyle(role: AgentRole): AgentStyleConfig {
  return AGENT_STYLES[role] || AGENT_STYLES.custom;
}

/**
 * 用户消息样式（固定）
 */
export const USER_MESSAGE_STYLE: AgentStyleConfig = {
  radius: 'rounded-2xl rounded-br-sm',
  color: '#52c41a', // 绿色用于用户 @提及
};

/**
 * 系统消息样式（固定）
 */
export const SYSTEM_MESSAGE_STYLE: AgentStyleConfig = {
  radius: 'rounded-lg',
  color: '#1890ff',
};