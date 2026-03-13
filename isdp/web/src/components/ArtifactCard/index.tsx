import React from 'react';
import { Card, Collapse, Tag, Typography, Space, Button, Tooltip } from 'antd';
import {
  FileTextOutlined,
  ApiOutlined,
  CodeOutlined,
  CheckCircleOutlined,
  FilePdfOutlined,
  FileMarkdownOutlined,
} from '@ant-design/icons';
import type { Artifact } from '@/types';

const { Panel } = Collapse;
const { Text } = Typography;

interface ArtifactCardProps {
  artifact: Artifact;
  onClick?: (artifact: Artifact) => void;
  onPreview?: (artifact: Artifact) => void;
}

/**
 * 单个产物卡片组件
 */
export const ArtifactCardItem: React.FC<ArtifactCardProps> = ({
  artifact,
  onClick,
  onPreview,
}) => {
  const getIcon = (type: string) => {
    switch (type) {
      case 'document':
        return <FileTextOutlined />;
      case 'code':
        return <CodeOutlined />;
      case 'review':
        return <CheckCircleOutlined />;
      case 'config':
        return <ApiOutlined />;
      default:
        return <FileTextOutlined />;
    }
  };

  const getColor = (type: string) => {
    switch (type) {
      case 'document':
        return 'blue';
      case 'code':
        return 'green';
      case 'review':
        return 'orange';
      case 'config':
        return 'purple';
      default:
        return 'default';
    }
  };

  const getFileIcon = (path?: string) => {
    if (!path) return <FileTextOutlined />;
    if (path.endsWith('.pdf')) return <FilePdfOutlined />;
    if (path.endsWith('.md')) return <FileMarkdownOutlined />;
    if (path.endsWith('.ts') || path.endsWith('.tsx')) return <CodeOutlined />;
    if (path.endsWith('.go')) return <CodeOutlined />;
    return <FileTextOutlined />;
  };

  return (
    <Card
      size="small"
      hoverable
      className="artifact-card-item"
      onClick={() => onClick?.(artifact)}
      style={{ marginBottom: 8 }}
    >
      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
        <Space>
          <span style={{ fontSize: 20, color: getColor(artifact.type) }}>
            {getIcon(artifact.type)}
          </span>
          <div>
            <Text strong style={{ display: 'block' }}>
              {artifact.name}
            </Text>
            {artifact.path && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                {artifact.path}
              </Text>
            )}
          </div>
        </Space>
        <Space>
          <Tag color={getColor(artifact.type)}>
            {artifact.type}
          </Tag>
          <Tooltip title="预览">
            <Button
              type="text"
              icon={getFileIcon(artifact.path)}
              onClick={(e) => {
                e.stopPropagation();
                onPreview?.(artifact);
              }}
              size="small"
            />
          </Tooltip>
        </Space>
      </Space>
    </Card>
  );
};

interface ArtifactPanelProps {
  artifacts: Artifact[];
  visible?: boolean;
  onArtifactClick?: (artifact: Artifact) => void;
  onPreview?: (artifact: Artifact) => void;
}

/**
 * 侧边产物面板组件
 */
export const ArtifactPanel: React.FC<ArtifactPanelProps> = ({
  artifacts,
  visible = true,
  onArtifactClick,
  onPreview,
}) => {
  const [expandedKeys, setExpandedKeys] = React.useState<string[]>(['all']);

  // 按类型分组
  const groupedArtifacts = artifacts.reduce((acc, artifact) => {
    if (!acc[artifact.type]) {
      acc[artifact.type] = [];
    }
    acc[artifact.type].push(artifact);
    return acc;
  }, {} as Record<string, Artifact[]>);

  const getTypeLabel = (type: string) => {
    const labels: Record<string, string> = {
      document: '文档',
      code: '代码',
      review: '评审报告',
      config: '配置文件',
      test: '测试文件',
    };
    return labels[type] || type;
  };

  if (!visible || artifacts.length === 0) {
    return null;
  }

  return (
    <div className="artifact-panel" style={{
      width: 300,
      borderLeft: '1px solid #f0f0f0',
      overflowY: 'auto',
      background: '#fafafa'
    }}>
      <div style={{ padding: 12, borderBottom: '1px solid #f0f0f0' }}>
        <Text strong>工作产物</Text>
        <Tag style={{ marginLeft: 8 }}>{artifacts.length}</Tag>
      </div>

      <Collapse
        activeKey={expandedKeys}
        onChange={(keys) => setExpandedKeys(keys as string[])}
        ghost
      >
        <Panel header="全部产物" key="all">
          {artifacts.map((artifact) => (
            <ArtifactCardItem
              key={artifact.id}
              artifact={artifact}
              onClick={onArtifactClick}
              onPreview={onPreview}
            />
          ))}
        </Panel>

        {Object.entries(groupedArtifacts).map(([type, items]) => (
          <Panel
            key={type}
            header={`${getTypeLabel(type)} (${items.length})`}
          >
            {items.map((artifact) => (
              <ArtifactCardItem
                key={artifact.id}
                artifact={artifact}
                onClick={onArtifactClick}
                onPreview={onPreview}
              />
            ))}
          </Panel>
        ))}
      </Collapse>
    </div>
  );
};

export default ArtifactPanel;
