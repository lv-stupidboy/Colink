import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Progress, Table, Tag, Space, Typography, Button } from 'antd';
import {
  ProjectOutlined,
  ThunderboltOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  PlusOutlined,
  ArrowRightOutlined,
  TeamOutlined,
  RobotOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { Project, Thread, WorkflowTemplate, AgentConfig } from '@/types';
import { PhaseLabels } from '@/types';

const { Title, Text } = Typography;

const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeThreads, setActiveThreads] = useState<Thread[]>([]);
  const [stats, setStats] = useState({
    totalProjects: 0,
    activeThreads: 0,
    completedThreads: 0,
    pendingReviews: 0,
  });
  const [loading, setLoading] = useState(false);
  const [workflows, setWorkflows] = useState<WorkflowTemplate[]>([]);
  const [agentConfigs, setAgentConfigs] = useState<AgentConfig[]>([]);

  useEffect(() => {
    loadDashboardData();
  }, []);

  const loadDashboardData = async () => {
    setLoading(true);
    try {
      // 并行加载所有数据
      const [projectData, workflowsData, agentsData] = await Promise.all([
        api.projects.list(),
        api.workflows.list(),
        api.agents.list(),
      ]);

      // 处理项目列表
      const projectsList = ((projectData as unknown as Project[]) || []);
      setProjects(projectsList.slice(0, 5));

      // 统计
      setStats({
        totalProjects: projectsList.length,
        activeThreads: 3, // 模拟数据
        completedThreads: 12,
        pendingReviews: 2,
      });

      // 设置工作流和Agent数据
      setWorkflows(workflowsData || []);
      setAgentConfigs(agentsData || []);

      // 加载活跃线程
      setActiveThreads([
        {
          id: '1',
          projectId: '1',
          name: '功能开发中',
          status: 'running',
          currentPhase: 'development',
          currentAgent: 'developer',
          depth: 2,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        {
          id: '2',
          projectId: '2',
          name: '代码审查中',
          status: 'running',
          currentPhase: 'review',
          currentAgent: 'reviewer',
          depth: 1,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ]);
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    } finally {
      setLoading(false);
    }
  };

  const projectColumns = [
    {
      title: '项目名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: Project) => (
        <a onClick={() => navigate(`/projects/${record.id}`)}>{name}</a>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'active' ? 'green' : 'default'}>
          {status === 'active' ? '活跃' : '归档'}
        </Tag>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type?: string) => {
        const typeMap: Record<string, string> = {
          service: '服务',
          app: '应用',
          task: '任务',
        };
        return typeMap[type || 'service'] || type || '服务项目';
      },
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      render: (date: string) => new Date(date).toLocaleString(),
    },
  ];

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12 }}>
        <Title level={2} style={{ margin: 0 }}>Dashboard</Title>
        <Text type="secondary">Welcome to Colink</Text>
      </div>

      {/* 统计卡片 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="项目总数"
              value={stats.totalProjects}
              prefix={<ProjectOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="进行中任务"
              value={stats.activeThreads}
              prefix={<ClockCircleOutlined />}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="已完成任务"
              value={stats.completedThreads}
              prefix={<CheckCircleOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="待审查"
              value={stats.pendingReviews}
              prefix={<ThunderboltOutlined />}
              valueStyle={{ color: '#eb2f96' }}
            />
          </Card>
        </Col>
      </Row>

      {/* Agent团队和角色卡片 */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={12}>
          <Card
            hoverable
            onClick={() => navigate('/workflow')}
            style={{ cursor: 'pointer' }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <Space>
                <TeamOutlined style={{ fontSize: 18 }} />
                <Text strong>Agent团队</Text>
                <Tag color="blue">{workflows.length}</Tag>
              </Space>
              <ArrowRightOutlined />
            </div>
            {workflows.length > 0 ? (
              <>
                <div style={{ marginBottom: 8 }}>
                  {workflows.slice(0, 3).map((workflow) => (
                    <div key={workflow.id} style={{ padding: '4px 0', color: 'var(--text-secondary)' }}>
                      · {workflow.name} ({workflow.agentIds?.length || 0} Agents)
                    </div>
                  ))}
                </div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  查看全部 →
                </Text>
              </>
            ) : (
              <Text type="secondary">暂无团队，点击创建</Text>
            )}
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card
            hoverable
            onClick={() => navigate('/agents')}
            style={{ cursor: 'pointer' }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <Space>
                <RobotOutlined style={{ fontSize: 18 }} />
                <Text strong>Agent角色</Text>
                <Tag color="purple">{agentConfigs.length}</Tag>
              </Space>
              <ArrowRightOutlined />
            </div>
            {agentConfigs.length > 0 ? (
              <>
                <div style={{ marginBottom: 8 }}>
                  <div style={{ padding: '4px 0', color: 'var(--text-secondary)' }}>
                    系统预置: {agentConfigs.filter(a => a.isSystem).length}
                  </div>
                  <div style={{ padding: '4px 0', color: 'var(--text-secondary)' }}>
                    自定义: {agentConfigs.filter(a => !a.isSystem).length}
                  </div>
                </div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  查看全部 →
                </Text>
              </>
            ) : (
              <Text type="secondary">暂无角色，点击创建</Text>
            )}
          </Card>
        </Col>
      </Row>

      {/* 活跃线程 */}
      <Title level={4} style={{ marginTop: 32 }}>活跃任务</Title>
      <Row gutter={[16, 16]}>
        {activeThreads.map((thread) => (
          <Col xs={24} lg={12} key={thread.id}>
            <Card
              hoverable
              onClick={() => navigate(`/projects/${thread.projectId}/threads/${thread.id}`)}
              extra={<ArrowRightOutlined />}
            >
              <Space direction="vertical" style={{ width: '100%' }} size="small">
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Text strong>Thread #{thread.id.slice(0, 8)}</Text>
                  <Tag color="processing">{thread.status}</Tag>
                </div>
                <Progress
                  percent={getPhaseProgress(thread.currentPhase)}
                  status="active"
                  size="small"
                />
                <Space split={<span style={{ color: '#d9d9d9' }}>/</span>}>
                  <Text type="secondary">阶段:</Text>
                  <Tag color="blue">{PhaseLabels[thread.currentPhase]}</Tag>
                </Space>
                <Space split={<span style={{ color: '#d9d9d9' }}>/</span>}>
                  <Text type="secondary">当前 Agent:</Text>
                  <Tag>{thread.currentAgent}</Tag>
                </Space>
              </Space>
            </Card>
          </Col>
        ))}
      </Row>

      {/* 最近项目 */}
      <Title level={4} style={{ marginTop: 32 }}>
        <Space>
          最近项目
          <Button type="link" icon={<PlusOutlined />} onClick={() => navigate('/projects')}>
            新建项目
          </Button>
        </Space>
      </Title>
      <Card>
        <Table
          dataSource={projects}
          columns={projectColumns}
          rowKey="id"
          loading={loading}
          pagination={false}
        />
      </Card>
    </div>
  );
};

// 根据阶段返回进度百分比
const getPhaseProgress = (phase: string): number => {
  const phases = ['requirement', 'design', 'development', 'review', 'test', 'merge', 'complete'];
  const index = phases.indexOf(phase);
  return Math.round(((index + 1) / phases.length) * 100);
};

export default Dashboard;
