import React, { useEffect, useState, useRef } from 'react';
import { Card, Table, Tag, Space, Button, Modal, message, Drawer, Typography, Descriptions, Statistic, Row, Col, Tabs, Input, Alert, Empty } from 'antd';
import {
  InboxOutlined,
  PlayCircleOutlined,
  StopOutlined,
  FileTextOutlined,
  ReloadOutlined,
  ClearOutlined,
  CloudServerOutlined,
  CodeOutlined,
  ExpandOutlined,
} from '@ant-design/icons';
import api from '@/api/client';

const { Title, Text } = Typography;
const { TabPane } = Tabs;
const { Search } = Input;

interface Sandbox {
  id: string;
  threadId: string;
  projectPath: string;
  mode: string;
  port: number;
  url: string;
  status: string;
  containerId?: string;
  startedAt: string;
}

interface LogEntry {
  timestamp: string;
  level: 'info' | 'warn' | 'error' | 'debug';
  message: string;
  source?: string;
}

/**
 * 沙箱环境页面
 * PRD Section 2.1.7 - 沙箱部署功能
 *
 * 功能要点：
 * - 本地沙箱环境启动
 * - 环境自动配置
 * - 服务一键部署
 * - 实时日志查看
 * - 资源使用监控
 */
const SandboxPage: React.FC = () => {
  const [sandboxes, setSandboxes] = useState<Sandbox[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedSandbox, setSelectedSandbox] = useState<Sandbox | null>(null);
  const [detailDrawerVisible, setDetailDrawerVisible] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logSearchText, setLogSearchText] = useState('');
  const logContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadSandboxes();
    // 定时刷新，每5秒
    const interval = setInterval(loadSandboxes, 5000);
    return () => clearInterval(interval);
  }, []);

  // 自动滚动日志到底部
  useEffect(() => {
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs]);

  const loadSandboxes = async () => {
    setLoading(true);
    try {
      const data = await api.sandbox.listServers();
      setSandboxes(data);
    } catch (error) {
      message.error('加载沙箱列表失败');
    } finally {
      setLoading(false);
    }
  };

  const loadLogs = async (sandboxId: string) => {
    setLogsLoading(true);
    try {
      const result = await api.sandbox.getLogs(sandboxId);
      // 解析日志文本为日志条目
      const logLines = (result.logs || '').split('\n').filter((line: string) => line.trim());
      const logEntries: LogEntry[] = logLines.map((line: string) => ({
        timestamp: new Date().toISOString(),
        level: 'info' as const,
        message: line,
        source: selectedSandbox?.mode === 'docker' ? 'docker' : 'local',
      }));
      setLogs(logEntries);
    } catch (error) {
      message.error('加载日志失败');
    } finally {
      setLogsLoading(false);
    }
  };

  const handleOpenPreview = (sandbox: Sandbox) => {
    if (sandbox.url) {
      window.open(sandbox.url, '_blank');
    }
  };

  const handleStop = async (sandboxId: string) => {
    Modal.confirm({
      title: '确认停止？',
      content: '停止沙箱将终止所有运行中的进程',
      okText: '确认停止',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await api.sandbox.stopServer(sandboxId);
          message.success('沙箱已停止');
          loadSandboxes();
        } catch (error) {
          message.error('停止沙箱失败');
        }
      },
    });
  };

  const handleViewDetails = (sandbox: Sandbox) => {
    setSelectedSandbox(sandbox);
    setDetailDrawerVisible(true);
    loadLogs(sandbox.id);
  };

  const handleClearLogs = () => {
    setLogs([]);
  };

  const handleRefreshLogs = () => {
    if (selectedSandbox) {
      loadLogs(selectedSandbox.id);
    }
  };

  /**
   * 渲染日志内容
   */
  const renderLogs = () => {
    const filteredLogs = logs.filter((log) =>
      logSearchText
        ? log.message.toLowerCase().includes(logSearchText.toLowerCase()) ||
          log.source?.toLowerCase().includes(logSearchText.toLowerCase())
        : true
    );

    const getLogColor = (level: string) => {
      const colors: Record<string, string> = {
        info: '#1890ff',
        warn: '#faad14',
        error: '#ff4d4f',
        debug: '#8c8c8c',
      };
      return colors[level] || '#666';
    };

    return (
      <div
        ref={logContainerRef}
        style={{
          background: '#1e1e1e',
          borderRadius: 8,
          padding: 16,
          maxHeight: 400,
          overflow: 'auto',
          fontFamily: 'Consolas, Monaco, monospace',
          fontSize: 12,
        }}
      >
        {filteredLogs.length === 0 ? (
          <Empty description="暂无日志" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
          filteredLogs.map((log, index) => (
            <div key={index} style={{ marginBottom: 4 }}>
              <span style={{ color: '#8c8c8c' }}>
                [{new Date(log.timestamp).toLocaleTimeString()}]
              </span>
              <span style={{ color: getLogColor(log.level), marginLeft: 8 }}>
                [{log.level.toUpperCase()}]
              </span>
              {log.source && (
                <span style={{ color: '#52c41a', marginLeft: 8 }}>
                  [{log.source}]
                </span>
              )}
              <span style={{ color: '#d4d4d4', marginLeft: 8 }}>{log.message}</span>
            </div>
          ))
        )}
      </div>
    );
  };

  const columns = [
    {
      title: '项目路径',
      dataIndex: 'projectPath',
      key: 'projectPath',
      render: (projectPath: string) => (
        <Space>
          <CloudServerOutlined />
          <Text strong>{projectPath}</Text>
        </Space>
      ),
    },
    {
      title: '模式',
      dataIndex: 'mode',
      key: 'mode',
      render: (mode: string) => (
        <Tag color={mode === 'docker' ? 'blue' : 'green'}>
          {mode === 'docker' ? '容器' : '本地'}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const colorMap: Record<string, string> = {
          running: 'green',
          stopped: 'default',
          starting: 'blue',
          error: 'red',
        };
        const textMap: Record<string, string> = {
          running: '运行中',
          stopped: '已停止',
          starting: '启动中',
          error: '错误',
        };
        return <Tag color={colorMap[status] || 'default'}>{textMap[status] || status}</Tag>;
      },
    },
    {
      title: '端口',
      dataIndex: 'port',
      key: 'port',
      render: (port?: number) => port ? `:${port}` : '-',
    },
    {
      title: '预览地址',
      dataIndex: 'url',
      key: 'url',
      render: (url: string) => url ? (
        <a href={url} target="_blank" rel="noopener noreferrer">{url}</a>
      ) : '-',
    },
    {
      title: '启动时间',
      dataIndex: 'startedAt',
      key: 'startedAt',
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: Sandbox) => (
        <Space>
          {record.status === 'running' && (
            <>
              <Button
                size="small"
                icon={<ExpandOutlined />}
                onClick={() => handleOpenPreview(record)}
              >
                预览
              </Button>
              <Button
                danger
                size="small"
                icon={<StopOutlined />}
                onClick={() => handleStop(record.id)}
              >
                停止
              </Button>
            </>
          )}
          <Button
            size="small"
            icon={<FileTextOutlined />}
            onClick={() => handleViewDetails(record)}
          >
            详情
          </Button>
        </Space>
      ),
    },
  ];

  // 计算统计
  const runningCount = sandboxes.filter((s) => s.status === 'running').length;
  const dockerCount = sandboxes.filter((s) => s.mode === 'docker').length;
  const localCount = sandboxes.filter((s) => s.mode === 'local').length;

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12 }}>
        <Title level={2} style={{ margin: 0 }}>沙箱环境</Title>
        <Text type="secondary">管理和监控隔离的执行环境</Text>
      </div>

      {/* 概览统计 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 12 }}>
        <Col xs={24} sm={6}>
          <Card>
            <Statistic
              title="运行中"
              value={runningCount}
              valueStyle={{ color: '#52c41a' }}
              prefix={<PlayCircleOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={6}>
          <Card>
            <Statistic
              title="已停止"
              value={sandboxes.filter((s) => s.status === 'stopped').length}
              valueStyle={{ color: '#999' }}
              prefix={<StopOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={6}>
          <Card>
            <Statistic
              title="容器沙箱"
              value={dockerCount}
              valueStyle={{ color: '#1890ff' }}
              prefix={<CloudServerOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={6}>
          <Card>
            <Statistic
              title="本地沙箱"
              value={localCount}
              valueStyle={{ color: '#722ed1' }}
            />
          </Card>
        </Col>
      </Row>

      {/* 沙箱列表 */}
      <Card>
        <Table
          dataSource={sandboxes}
          columns={columns}
          rowKey="id"
          loading={loading}
        />
      </Card>

      {/* 详情抽屉 */}
      <Drawer
        title={
          <Space>
            <CloudServerOutlined />
            <span>沙箱详情</span>
          </Space>
        }
        placement="right"
        width={700}
        open={detailDrawerVisible}
        onClose={() => setDetailDrawerVisible(false)}
      >
        {selectedSandbox && (
          <Tabs defaultActiveKey="info">
            <TabPane
              tab={
                <span>
                  <InboxOutlined />
                  基本信息
                </span>
              }
              key="info"
            >
              <Descriptions column={2} bordered size="small">
                <Descriptions.Item label="项目路径">{selectedSandbox.projectPath}</Descriptions.Item>
                <Descriptions.Item label="状态">
                  <Tag color={selectedSandbox.status === 'running' ? 'green' : 'default'}>
                    {selectedSandbox.status}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="运行模式">
                  <Tag color={selectedSandbox.mode === 'docker' ? 'blue' : 'green'}>
                    {selectedSandbox.mode === 'docker' ? '容器沙箱' : '本地沙箱'}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="端口">{selectedSandbox.port || '-'}</Descriptions.Item>
                <Descriptions.Item label="预览地址" span={2}>
                  {selectedSandbox.url ? (
                    <a href={selectedSandbox.url} target="_blank" rel="noopener noreferrer">{selectedSandbox.url}</a>
                  ) : '-'}
                </Descriptions.Item>
                <Descriptions.Item label="容器 ID" span={2}>
                  <Text code>{selectedSandbox.containerId || '-'}</Text>
                </Descriptions.Item>
                <Descriptions.Item label="Thread">{selectedSandbox.threadId}</Descriptions.Item>
                <Descriptions.Item label="启动时间">
                  {new Date(selectedSandbox.startedAt).toLocaleString()}
                </Descriptions.Item>
              </Descriptions>
            </TabPane>

            <TabPane
              tab={
                <span>
                  <CodeOutlined />
                  日志查看
                </span>
              }
              key="logs"
            >
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                <Space>
                  <Search
                    placeholder="搜索日志..."
                    allowClear
                    style={{ width: 200 }}
                    value={logSearchText}
                    onChange={(e) => setLogSearchText(e.target.value)}
                  />
                  <Button icon={<ReloadOutlined />} onClick={handleRefreshLogs} loading={logsLoading}>
                    刷新
                  </Button>
                  <Button icon={<ClearOutlined />} onClick={handleClearLogs}>
                    清空
                  </Button>
                </Space>

                {selectedSandbox.status !== 'running' && (
                  <Alert
                    type="info"
                    message="沙箱已停止，日志为历史记录"
                    showIcon
                  />
                )}

                {renderLogs()}
              </Space>
            </TabPane>
          </Tabs>
        )}
      </Drawer>
    </div>
  );
};

export default SandboxPage;