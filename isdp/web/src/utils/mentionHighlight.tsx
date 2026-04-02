// isdp/web/src/utils/mentionHighlight.tsx
import React from 'react';
import type { AgentConfig, AgentRole } from '@/types';
import { getAgentStyle } from '@/config/agentStyles';

/**
 * @提及高亮工具
 * 用于识别和高亮消息中的 @Agent 名字
 */

/**
 * 获取 @提及的正则表达式
 * 匹配 @后面跟着非空白字符的内容
 */
export function getMentionRegex(): RegExp {
  return /@([^\s@]+)/g;
}

/**
 * 从提及文本中提取 Agent 名字
 * @param mentionText 如 "@架构师" 或 "@Claude"
 * @returns Agent 名字（去掉 @ 前缀）
 */
export function extractMentionName(mentionText: string): string {
  return mentionText.startsWith('@') ? mentionText.slice(1) : mentionText;
}

/**
 * 根据 Agent 名字查找对应的 Agent 配置
 * @param name Agent 名字
 * @param agentConfigs Agent 配置列表
 * @returns 匹配的 Agent 配置，或 undefined
 */
export function findAgentByName(
  name: string,
  agentConfigs: AgentConfig[]
): AgentConfig | undefined {
  // 精确匹配
  const exactMatch = agentConfigs.find(
    (config) => config.name === name || config.name.includes(name)
  );

  // 如果精确匹配失败，尝试部分匹配（处理中文简称）
  if (!exactMatch) {
    return agentConfigs.find((config) => {
      const configName = config.name.toLowerCase();
      const searchName = name.toLowerCase();
      return configName.includes(searchName) || searchName.includes(configName);
    });
  }

  return exactMatch;
}

/**
 * 获取 Agent 的颜色
 * @param role Agent 角色
 * @returns 颜色值
 */
export function getAgentColor(role: AgentRole): string {
  return getAgentStyle(role).color;
}

/**
 * 高亮文本中的 @提及
 * @param text 原始文本
 * @param agentConfigs Agent 配置列表（用于颜色匹配）
 * @returns 渲染结果，包含高亮的 @提及
 */
export function highlightMentions(
  text: string,
  agentConfigs: AgentConfig[]
): React.ReactNode[] {
  const regex = getMentionRegex();
  const parts: React.ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let keyIndex = 0;

  while ((match = regex.exec(text)) !== null) {
    // 添加匹配前的文本
    if (match.index > lastIndex) {
      parts.push(text.slice(lastIndex, match.index));
    }

    // 处理 @提及
    const mentionText = match[0]; // 完整的 @提及，如 "@架构师"
    const agentName = match[1];   // Agent 名字，如 "架构师"

    // 查找对应的 Agent 配置
    const agentConfig = findAgentByName(agentName, agentConfigs);

    if (agentConfig) {
      // 找到匹配的 Agent，使用其角色颜色
      const color = getAgentColor(agentConfig.role);
      parts.push(
        <span
          key={`mention-${keyIndex++}`}
          className="mention-highlight"
          style={{
            backgroundColor: `${color}20`, // 20% 透明度背景
            color: color,
            padding: '2px 4px',
            borderRadius: '4px',
            fontWeight: 500,
          }}
          title={`${agentConfig.name} (${agentConfig.role})`}
        >
          {mentionText}
        </span>
      );
    } else {
      // 未找到匹配的 Agent，使用默认样式
      parts.push(
        <span
          key={`mention-${keyIndex++}`}
          className="mention-highlight mention-unknown"
          style={{
            backgroundColor: '#f0f0f0',
            color: '#595959',
            padding: '2px 4px',
            borderRadius: '4px',
          }}
        >
          {mentionText}
        </span>
      );
    }

    lastIndex = regex.lastIndex;
  }

  // 添加剩余文本
  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex));
  }

  // 如果没有匹配，返回原始文本
  if (parts.length === 0) {
    return [text];
  }

  return parts;
}

/**
 * 检查文本是否包含 @提及
 * @param text 文本内容
 * @returns 是否包含 @提及
 */
export function hasMentions(text: string): boolean {
  return getMentionRegex().test(text);
}