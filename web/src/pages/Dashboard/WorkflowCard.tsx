import React from 'react';
import { Card, Typography, Button, Tag } from 'antd';
import {
  TeamOutlined,
  RobotOutlined,
  BookOutlined,
  CodeOutlined,
  ApiOutlined,
  SafetyCertificateOutlined,
  DownOutlined,
  UpOutlined,
} from '@ant-design/icons';

const { Text } = Typography;

// Agent 信息类型定义
interface AgentInfo {
  id: string;
  name: string;
  role: string;
  skillsCount: number;
  commandsCount: number;
  subagentsCount: number;
  rulesCount: number;
  skills?: string[];
  commands?: string[];
  subagents?: string[];
  rules?: string[];
}

// WorkflowCard Props
interface WorkflowCardProps {
  id: string;
  name: string;
  description?: string;
  isSystem?: boolean;
  agents: AgentInfo[];
  skills: number;
  commands: number;
  subagents: number;
  rules: number;
  totalAssets: number;
}

// 资产类型配置 - 每种资产有独特颜色
const ASSET_CONFIG = {
  skills: { icon: <BookOutlined />, color: '#10b981', bg: 'rgba(16, 185, 129, 0.1)' },
  commands: { icon: <CodeOutlined />, color: '#8b5cf6', bg: 'rgba(139, 92, 246, 0.1)' },
  subagents: { icon: <ApiOutlined />, color: '#f59e0b', bg: 'rgba(245, 158, 11, 0.1)' },
  rules: { icon: <SafetyCertificateOutlined />, color: '#ef4444', bg: 'rgba(239, 68, 68, 0.1)' },
};

// Agent 角色卡片 - expanded由父组件控制
const AgentRowCard: React.FC<{ agent: AgentInfo; showDetails: boolean }> = ({ agent, showDetails }) => {
  return (
    <div
      style={{
        background: 'var(--bg-container)',
        border: '1px solid var(--border-color)',
        borderRadius: 6,
        padding: '10px 12px',
        marginBottom: 8,
      }}
    >
      {/* 第一行：角色名称 */}
      <div
        style={{
          background: 'linear-gradient(135deg, var(--color-section-orange-bg) 0%, var(--bg-container) 100%)',
          borderRadius: 4,
          padding: '8px 10px',
          marginBottom: 10,
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          borderLeft: '2px solid var(--color-section-orange)',
        }}
      >
        <RobotOutlined style={{ color: 'var(--color-section-orange)', fontSize: 14 }} />
        <Text strong style={{ fontSize: 13, color: 'var(--text-primary)', lineHeight: 1.4 }}>
          {agent.name}
        </Text>
      </div>

      {/* 第二行：资产横向排列 - Tag样式 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, flexWrap: 'wrap' }}>
        {agent.skillsCount > 0 && (
          <Tag style={{ margin: 0, padding: '2px 8px', borderRadius: 4, background: ASSET_CONFIG.skills.bg, border: `1px solid ${ASSET_CONFIG.skills.color}`, color: ASSET_CONFIG.skills.color, fontSize: 11, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
            {ASSET_CONFIG.skills.icon} Skill {agent.skillsCount}
          </Tag>
        )}
        {agent.commandsCount > 0 && (
          <Tag style={{ margin: 0, padding: '2px 8px', borderRadius: 4, background: ASSET_CONFIG.commands.bg, border: `1px solid ${ASSET_CONFIG.commands.color}`, color: ASSET_CONFIG.commands.color, fontSize: 11, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
            {ASSET_CONFIG.commands.icon} Command {agent.commandsCount}
          </Tag>
        )}
        {agent.subagentsCount > 0 && (
          <Tag style={{ margin: 0, padding: '2px 8px', borderRadius: 4, background: ASSET_CONFIG.subagents.bg, border: `1px solid ${ASSET_CONFIG.subagents.color}`, color: ASSET_CONFIG.subagents.color, fontSize: 11, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
            {ASSET_CONFIG.subagents.icon} Subagent {agent.subagentsCount}
          </Tag>
        )}
        {agent.rulesCount > 0 && (
          <Tag style={{ margin: 0, padding: '2px 8px', borderRadius: 4, background: ASSET_CONFIG.rules.bg, border: `1px solid ${ASSET_CONFIG.rules.color}`, color: ASSET_CONFIG.rules.color, fontSize: 11, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
            {ASSET_CONFIG.rules.icon} Rule {agent.rulesCount}
          </Tag>
        )}
      </div>

      {/* 展开后显示资产名称列表 */}
      {showDetails && (
        <div style={{ marginTop: 10, background: 'var(--bg-elevated)', borderRadius: 4 }}>
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {agent.skills && agent.skills.map((name, idx) => (
              <Tag key={`skill-${idx}`} style={{ margin: 0, fontSize: 11, padding: '2px 8px', background: ASSET_CONFIG.skills.bg, color: ASSET_CONFIG.skills.color }}>{name}</Tag>
            ))}
            {agent.commands && agent.commands.map((name, idx) => (
              <Tag key={`cmd-${idx}`} style={{ margin: 0, fontSize: 11, padding: '2px 8px', background: ASSET_CONFIG.commands.bg, color: ASSET_CONFIG.commands.color }}>{name}</Tag>
            ))}
            {agent.subagents && agent.subagents.map((name, idx) => (
              <Tag key={`sub-${idx}`} style={{ margin: 0, fontSize: 11, padding: '2px 8px', background: ASSET_CONFIG.subagents.bg, color: ASSET_CONFIG.subagents.color }}>{name}</Tag>
            ))}
            {agent.rules && agent.rules.map((name, idx) => (
              <Tag key={`rule-${idx}`} style={{ margin: 0, fontSize: 11, padding: '2px 8px', background: ASSET_CONFIG.rules.bg, color: ASSET_CONFIG.rules.color }}>{name}</Tag>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

// 主组件 - 单一展开按钮控制所有
const WorkflowCard: React.FC<WorkflowCardProps> = ({
  name,
  isSystem,
  agents,
}) => {
  const agentList = agents || [];
  const [expanded, setExpanded] = React.useState(false);

  // 默认显示2个，展开后显示全部 + 资产详情
  const visibleCount = expanded ? agentList.length : Math.min(agentList.length, 2);
  const visibleAgents = agentList.slice(0, visibleCount);
  const hasMoreAgents = agentList.length > 2;

  // 检查是否有资产可以展开显示
  const hasAssets = agentList.some(a =>
    (a.skills && a.skills.length > 0) ||
    (a.commands && a.commands.length > 0) ||
    (a.subagents && a.subagents.length > 0) ||
    (a.rules && a.rules.length > 0)
  );

  return (
    <Card
      style={{
        borderRadius: 10,
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        minHeight: 120,
      }}
      styles={{
        body: { padding: 12 },
      }}
    >
      {/* 标题行 */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          marginBottom: 12,
          padding: '8px 12px',
          background: 'linear-gradient(135deg, var(--color-section-teal-bg) 0%, var(--bg-container) 100%)',
          borderRadius: 6,
          borderLeft: '3px solid var(--color-section-teal)',
        }}
      >
        <TeamOutlined style={{ color: 'var(--color-section-teal)', fontSize: 15, marginRight: 8 }} />
        <Text strong style={{ fontSize: 14, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 200 }}>
          {name}
        </Text>
        {isSystem && (
          <span style={{ fontSize: 10, color: 'var(--color-section-purple)', marginLeft: 6, padding: '2px 6px', background: 'var(--color-section-purple-bg)', borderRadius: 4 }}>系统</span>
        )}
      </div>

      {/* Agent列表 */}
      {agentList.length > 0 ? (
        <div>
          {visibleAgents.map((agent) => (
            <AgentRowCard key={agent.id} agent={agent} showDetails={expanded} />
          ))}

          {/* 展开/收起按钮 - 有更多角色或有资产详情时显示 */}
          {(hasMoreAgents || hasAssets) && (
            <Button
              type="text"
              size="small"
              onClick={() => setExpanded(!expanded)}
              style={{
                width: '100%',
                fontSize: 11,
                color: 'var(--color-primary)',
                padding: '4px 0',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              {expanded ? (
                <>
                  收起 <UpOutlined style={{ fontSize: 10, marginLeft: 4 }} />
                </>
              ) : (
                <>
                  {hasMoreAgents ? `展开 +${agentList.length - 2}` : '展开详情'} <DownOutlined style={{ fontSize: 10, marginLeft: 4 }} />
                </>
              )}
            </Button>
          )}
        </div>
      ) : (
        <div style={{ padding: '16px 0', textAlign: 'center' }}>
          <Text type="secondary" style={{ fontSize: 12 }}>暂无角色</Text>
        </div>
      )}
    </Card>
  );
};

export default WorkflowCard;