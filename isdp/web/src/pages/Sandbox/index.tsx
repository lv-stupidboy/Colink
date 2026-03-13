import React, { useEffect, useState, useRef } from 'react';
import { Card, Table, Tag, Space, Button, Modal, message, Drawer, Typography, Descriptions, Statistic, Row, Col, Progress, Tabs, Input, Alert, Empty } from 'antd';
import {
  InboxOutlined,
  PlayCircleOutlined,
  StopOutlined,
  FileTextOutlined,
  DashboardOutlined,
  ReloadOutlined,
  ClearOutlined,
  CloudServerOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import api from '@/api/client';

const { Title, Text } = Typography;
const { TabPane } = Tabs;
const { Search } = Input;

interface Sandbox {
  id: string;
  threadId: string;
  name: string;
  image: string;
  status: 'created' | 'running' | 'stopped' | 'error';
  containerId?: string;
  port?: number;
  cpu?: number;
  memory?: number;
  disk?: number;
  createdAt: string;
  endedAt?: string;
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
      // TODO: 调用实际 API
      // const data = await api.sandbox.list();
      // 模拟数据
      setSandboxes([
        {
          id: '1',
          threadId: 'thread-1',
          name: 'dev-sandbox-1',
          image: 'node:18-alpine',
          status: 'running',
          containerId: 'abc123def456',
          port: 3000,
          cpu: 25,
          memory: 512,
          disk: 256,
          createdAt: new Date().toISOString(),
        },
        {
          id: '2',
          threadId: 'thread-2',
          name: 'test-sandbox-1',
          image: 'python:3.11-slim',
          status: 'stopped',
          containerId: 'def456ghi789',
          port: 8000,
          cpu: 0,
          memory: 0,
          disk: 0,
          createdAt: new Date(Date.now() - 3600000).toISOString(),
          endedAt: new Date().toISOString(),
        },
        {
          id: '3',
          threadId: 'thread-3',
          name: 'api-sandbox-1',
          image: 'golang:1.21-alpine',
          status: 'running',
          containerId: 'ghi789jkl012',
          port: 8080,
          cpu: 45,
          memory: 1024,
          disk: 512,
          createdAt: new Date(Date.now() - 7200000).toISOString(),
        },
      ]);
    } catch (error) {
      message.error('加载沙箱列表失败');
    } finally {
      setLoading(false);
    }
  };

  const loadLogs = async (_sandboxId: string) => {
    setLogsLoading(true);
    try {
      // TODO: 调用实际 API
      // const data = await api.sandbox.logs(sandboxId);
      // 模拟日志数据
      const mockLogs: LogEntry[] = [
        { timestamp: new Date().toISOString(), level: 'info', message: '容器启动中...', source: 'system' },
        { timestamp: new Date().toISOString(), level: 'info', message: '正在拉取镜像 node:18-alpine', source: 'docker' },
        { timestamp: new Date().toISOString(), level: 'info', message: '镜像拉取完成', source: 'docker' },
        { timestamp: new Date().toISOString(), level: 'info', message: '正在启动应用...', source: 'app' },
        { timestamp: new Date().toISOString(), level: 'debug', message: 'Environment: NODE_ENV=development', source: 'app' },
        { timestamp: new Date().toISOString(), level: 'info', message: 'Server listening on port 3000', source: 'app' },
        { timestamp: new Date().toISOString(), level: 'warn', message: 'Memory usage is above 80%', source: 'monitor' },
        { timestamp: new Date().toISOString(), level: 'info', message: 'GET /api/health - 200 OK (5ms)', source: 'http' },
        { timestamp: new Date().toISOString(), level: 'info', message: 'POST /api/users - 201 Created (45ms)', source: 'http' },
        { timestamp: new Date().toISOString(), level: 'error', message: 'Connection timeout to database', source: 'db' },
      ];
      setLogs(mockLogs);
    } catch (error) {
      message.error('加载日志失败');
    } finally {
      setLogsLoading(false);
    }
  };

  const handleStart = async (sandbox: Sandbox) => {
    try {
      await api.sandbox.run(sandbox.threadId, { image: sandbox.image, command: [] });
      message.success('沙箱启动成功');
      loadSandboxes();
    } catch (error) {
      message.error('启动沙箱失败');
    }
  };

  const handleStop = async (_sandboxId: string) => {
    Modal.confirm({
      title: '确认停止？',
      content: '停止沙箱将终止所有运行中的进程',
      okText: '确认停止',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        // TODO: 调用停止 API
        message.success('沙箱已停止');
        loadSandboxes();
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
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => (
        <Space>
          <CloudServerOutlined />
          <Text strong>{name}</Text>
        </Space>
      ),
    },
    {
      title: '镜像',
      dataIndex: 'image',
      key: 'image',
      render: (image: string) => <Text code>{image}</Text>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const colorMap: Record<string, string> = {
          running: 'green',
          stopped: 'default',
          created: 'blue',
          error: 'red',
        };
        const textMap: Record<string, string> = {
          running: '运行中',
          stopped: '已停止',
          created: '已创建',
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
      title: 'CPU/内存',
      key: 'resources',
      render: (_: unknown, record: Sandbox) =>
        record.status === 'running' ? (
          <Space>
            <Text>{record.cpu}%</Text>
            <Text type="secondary">/</Text>
            <Text>{record.memory}MB</Text>
          </Space>
        ) : '-',
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: Sandbox) => (
        <Space>
          {record.status === 'running' ? (
            <Button
              danger
              size="small"
              icon={<StopOutlined />}
              onClick={() => handleStop(record.id)}
            >
              停止
            </Button>
          ) : (
            <Button
              type="primary"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => handleStart(record)}
            >
              启动
            </Button>
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
  const totalCpu = sandboxes.reduce((sum, s) => sum + (s.cpu || 0), 0);
  const totalMemory = sandboxes.reduce((sum, s) => sum + (s.memory || 0), 0);

  return (
    <div className="sandbox-page">
      <div style={{ marginBottom: 24 }}>
        <Title level={2}>
          <Space>
            <DashboardOutlined />
            沙箱环境
          </Space>
        </Title>
        <Text type="secondary">管理和监控隔离的执行环境</Text>
      </div>

      {/* 概览统计 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
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
              title="CPU 总使用"
              value={totalCpu}
              suffix="%"
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={6}>
          <Card>
            <Statistic
              title="内存总使用"
              value={totalMemory}
              suffix="MB"
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
                <Descriptions.Item label="名称">{selectedSandbox.name}</Descriptions.Item>
                <Descriptions.Item label="状态">
                  <Tag color={selectedSandbox.status === 'running' ? 'green' : 'default'}>
                    {selectedSandbox.status}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="镜像" span={2}>
                  <Text code>{selectedSandbox.image}</Text>
                </Descriptions.Item>
                <Descriptions.Item label="容器 ID" span={2}>
                  <Text code>{selectedSandbox.containerId || '-'}</Text>
                </Descriptions.Item>
                <Descriptions.Item label="端口">{selectedSandbox.port || '-'}</Descriptions.Item>
                <Descriptions.Item label="Thread">{selectedSandbox.threadId}</Descriptions.Item>
                <Descriptions.Item label="创建时间">
                  {new Date(selectedSandbox.createdAt).toLocaleString()}
                </Descriptions.Item>
                <Descriptions.Item label="结束时间">
                  {selectedSandbox.endedAt ? new Date(selectedSandbox.endedAt).toLocaleString() : '-'}
                </Descriptions.Item>
              </Descriptions>

              {selectedSandbox.status === 'running' && (
                <Card title="资源监控" size="small" style={{ marginTop: 16 }}>
                  <Row gutter={16}>
                    <Col span={8}>
                      <Statistic
                        title="CPU"
                        value={selectedSandbox.cpu || 0}
                        suffix="%"
                        valueStyle={{ fontSize: 24 }}
                      />
                      <Progress
                        percent={selectedSandbox.cpu || 0}
                        strokeColor="#1890ff"
                        size="small"
                      />
                    </Col>
                    <Col span={8}>
                      <Statistic
                        title="内存"
                        value={selectedSandbox.memory || 0}
                        suffix="MB"
                        valueStyle={{ fontSize: 24 }}
                      />
                      <Progress
                        percent={Math.round(((selectedSandbox.memory || 0) / 2048) * 100)}
                        strokeColor="#52c41a"
                        size="small"
                      />
                    </Col>
                    <Col span={8}>
                      <Statistic
                        title="磁盘"
                        value={selectedSandbox.disk || 0}
                        suffix="MB"
                        valueStyle={{ fontSize: 24 }}
                      />
                      <Progress
                        percent={Math.round(((selectedSandbox.disk || 0) / 1024) * 100)}
                        strokeColor="#722ed1"
                        size="small"
                      />
                    </Col>
                  </Row>
                </Card>
              )}
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