import React, { useState, useEffect } from 'react';
import {
  Button,
  Card,
  Typography,
  Space,
  Table,
  Tag,
  Divider,
  Alert,
  message,
} from 'antd';
import {
  PlayCircleOutlined,
  StopOutlined,
  SettingOutlined,
  FileTextOutlined,
  FolderOutlined,
  GlobalOutlined,
} from '@ant-design/icons';
import { serviceApi, launcherApi, modeApi, installApi } from '../../../lib/api';
import type { RunningAgentInstance } from '../../../lib/api/types';

const { Title, Text } = Typography;

// Colink Logo SVG - 六边形网络设计（缩小版）
const ColinkLogo = () => (
  <svg width="48" height="48" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <linearGradient id="colinkGrad" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#10b981' }} />
        <stop offset="100%" style={{ stopColor: '#3b82f6' }} />
      </linearGradient>
    </defs>
    {/* 背景 */}
    <rect x="2" y="2" width="28" height="28" rx="6" fill="#0f172a" />
    {/* 六边形轮廓线 */}
    <polygon
      points="16,6 24,10.5 24,21.5 16,26 8,21.5 8,10.5"
      fill="none"
      stroke="#10b981"
      strokeWidth="1.2"
      strokeOpacity="0.35"
      strokeLinejoin="round"
    />
    {/* 从外环到中心的连接线 */}
    <g stroke="#10b981" strokeWidth="0.8" strokeOpacity="0.35">
      <line x1="16" y1="6" x2="16" y2="16" />
      <line x1="24" y1="10.5" x2="16" y2="16" />
      <line x1="24" y1="21.5" x2="16" y2="16" />
      <line x1="16" y1="26" x2="16" y2="16" />
      <line x1="8" y1="21.5" x2="16" y2="16" />
      <line x1="8" y1="10.5" x2="16" y2="16" />
    </g>
    {/* 外环节点 (6个) */}
    <circle cx="16" cy="6" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="24" cy="10.5" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="24" cy="21.5" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="16" cy="26" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="8" cy="21.5" r="1.8" fill="url(#colinkGrad)" />
    <circle cx="8" cy="10.5" r="1.8" fill="url(#colinkGrad)" />
    {/* 中心节点 */}
    <circle cx="16" cy="16" r="3" fill="url(#colinkGrad)" />
    {/* 节点高光 */}
    <circle cx="16" cy="6" r="0.7" fill="white" opacity="0.3" />
    <circle cx="24" cy="10.5" r="0.7" fill="white" opacity="0.3" />
    <circle cx="24" cy="21.5" r="0.7" fill="white" opacity="0.3" />
    <circle cx="16" cy="26" r="0.7" fill="white" opacity="0.3" />
    <circle cx="8" cy="21.5" r="0.7" fill="white" opacity="0.3" />
    <circle cx="8" cy="10.5" r="0.7" fill="white" opacity="0.3" />
    <circle cx="16" cy="16" r="1.2" fill="white" opacity="0.4" />
  </svg>
);

const LauncherDashboard: React.FC = () => {
  const [serviceStatus, setServiceStatus] = useState<'running' | 'stopped'>('stopped');
  const [runningAgents, setRunningAgents] = useState<RunningAgentInstance[]>([]);
  const [agentCount, setAgentCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [installDir, setInstallDir] = useState('');

  useEffect(() => {
    checkStatus();
    loadInstallDir();
    const interval = setInterval(checkStatus, 5000);
    return () => clearInterval(interval);
  }, []);

  const loadInstallDir = async () => {
    try {
      // 从 state 获取安装目录（Launcher 启动时已从注册表加载）
      const dir = await modeApi.getInstallDir();
      if (dir) {
        setInstallDir(dir);
      } else {
        // 如果 state 没有，尝试从注册表直接获取
        const result = await installApi.checkInstalled();
        if (result.installDir) {
          setInstallDir(result.installDir);
        } else {
          setInstallDir('未知');
        }
      }
    } catch (err) {
      console.error('Failed to get install dir:', err);
      setInstallDir('未知');
    }
  };

  const checkStatus = async () => {
    try {
      const status = await serviceApi.getStatus();
      setServiceStatus(status.status);

      // 始终获取运行中的 Agent（服务可能独立运行）
      const agents = await serviceApi.getRunningAgents();
      setRunningAgents(agents.instances || []);
      setAgentCount(agents.instances?.length || 0);
    } catch (err) {
      console.error('Failed to check status:', err);
      setRunningAgents([]);
      setAgentCount(0);
    }
  };

  const handleStart = async () => {
    setLoading(true);
    try {
      const result = await serviceApi.start();
      if (!result.success && result.error) {
        message.error(`启动失败: ${result.error}`);
      } else {
        await checkStatus();
      }
    } catch (err) {
      console.error('Failed to start service:', err);
      message.error('启动服务失败');
    } finally {
      setLoading(false);
    }
  };

  const handleStop = async () => {
    if (agentCount > 0) {
      message.warning('有 Agent 实例正在运行，请先在 Web 控制台停止');
      return;
    }
    setLoading(true);
    try {
      await serviceApi.stop();
      await checkStatus();
    } catch (err) {
      console.error('Failed to stop service:', err);
      message.error('停止服务失败');
    } finally {
      setLoading(false);
    }
  };

  const handleOpenConsole = async () => {
    try {
      await launcherApi.openConsole();
    } catch (err) {
      console.error('Failed to open console:', err);
    }
  };

  const handleOpenLogs = async () => {
    try {
      await launcherApi.openLogs();
    } catch (err) {
      console.error('Failed to open logs:', err);
    }
  };

  const handleOpenDataDir = async () => {
    try {
      await launcherApi.openDataDir();
    } catch (err) {
      console.error('Failed to open data dir:', err);
    }
  };

  const handleOpenConfig = async () => {
    try {
      await launcherApi.openConfig();
    } catch (err) {
      console.error('Failed to open config:', err);
    }
  };

  const formatDuration = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    if (mins < 1) return '<1分钟';
    if (mins < 60) return `${mins}分钟`;
    const hours = Math.floor(mins / 60);
    const remainMins = mins % 60;
    return `${hours}小时${remainMins}分钟`;
  };

  const columns = [
    {
      title: '项目',
      dataIndex: 'projectName',
      key: 'project',
      width: 100,
      ellipsis: true,
    },
    {
      title: '任务',
      dataIndex: 'threadTitle',
      key: 'thread',
      width: 150,
      ellipsis: true,
    },
    {
      title: 'Agent',
      dataIndex: 'agentName',
      key: 'agent',
      width: 120,
      ellipsis: true,
    },
    {
      title: '运行时间',
      key: 'duration',
      width: 80,
      render: (_: unknown, record: RunningAgentInstance) => formatDuration(record.runningDurationSeconds),
    },
  ];

  const isRunning = serviceStatus === 'running';

  return (
    <div style={{ padding: 24 }}>
      {/* Logo 和标题 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 16 }}>
        <ColinkLogo />
        <div>
          <Title level={3} style={{ margin: 0 }}>Colink</Title>
          <Text type="secondary">多智能体协作平台</Text>
        </div>
      </div>

      <Divider />

      {/* 服务状态 */}
      <Card size="small" style={{ marginBottom: 16 }} loading={loading}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <Text strong>服务状态：</Text>
            <Tag color={isRunning ? 'green' : 'default'}>
              {isRunning ? '运行中' : '已停止'}
            </Tag>
          </div>
          <Space>
            <Button
              icon={<GlobalOutlined />}
              onClick={handleOpenConsole}
              disabled={!isRunning}
            >
              打开控制台
            </Button>
            {isRunning ? (
              <Button
                icon={<StopOutlined />}
                onClick={handleStop}
                danger
                disabled={agentCount > 0}
              >
                停止服务
              </Button>
            ) : (
              <Button
                type="primary"
                icon={<PlayCircleOutlined />}
                onClick={handleStart}
              >
                启动服务
              </Button>
            )}
          </Space>
        </div>
      </Card>

      {/* Agent实例列表 */}
      <Card title="正在运行的Agent实例" size="small" style={{ marginBottom: 16 }}>
        {runningAgents.length === 0 ? (
          <Text type="secondary">当前无Agent实例运行</Text>
        ) : (
          <>
            <Table
              size="small"
              dataSource={runningAgents}
              rowKey="invocationId"
              pagination={false}
              columns={columns}
            />
            <Alert
              type="warning"
              showIcon
              style={{ marginTop: 8 }}
              message={`有${agentCount}个Agent实例正在运行，请在Web控制台手动停止后才能停止服务`}
            />
          </>
        )}
      </Card>

      {/* 快捷操作 */}
      <Card title="快捷操作" size="small" style={{ marginBottom: 16 }}>
        <Space wrap>
          <Button
            icon={<SettingOutlined />}
            onClick={handleOpenConfig}
          >
            系统配置
          </Button>
          <Button
            icon={<FileTextOutlined />}
            onClick={handleOpenLogs}
          >
            查看日志
          </Button>
          <Button
            icon={<FolderOutlined />}
            onClick={handleOpenDataDir}
          >
            数据目录
          </Button>
        </Space>
      </Card>

      {/* 安装信息 */}
      <Card title="安装信息" size="small">
        <div>
          <Text type="secondary">安装目录：</Text>
          <Text code>{installDir}</Text>
        </div>
      </Card>
    </div>
  );
};

export default LauncherDashboard;