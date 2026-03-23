import React, { useCallback, useMemo, useState } from 'react';
import {
  ReactFlow,
  Edge,
  Connection,
  useNodesState,
  useEdgesState,
  Controls,
  MiniMap,
  Background,
  BackgroundVariant,
  Handle,
  Position,
  Panel,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  Card,
  Button,
  Space,
  Tag,
  Select,
  Input,
  Form,
  Modal,
  message,
  Typography,
  Popconfirm,
  Tooltip,
} from 'antd';
import {
  SaveOutlined,
  DeleteOutlined,
  UserOutlined,
  ArrowRightOutlined,
  BranchesOutlined,
  MergeCellsOutlined,
} from '@ant-design/icons';
import type { AgentConfig, Transition, TransitionType } from '@/types';

const { Text } = Typography;
const { Option } = Select;
const { TextArea } = Input;

// Agent节点组件 - 使用泛型类型避免类型冲突
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const AgentNode: React.FC<any> = ({ data, selected }) => {
  const { agent, onDelete } = data || {};

  if (!agent) return null;

  const roleColors: Record<string, string> = {
    requirement: '#1890ff',
    architect: '#722ed1',
    developer: '#52c41a',
    reviewer: '#faad14',
    testengineer: '#eb2f96',
    devops: '#13c2c2',
    custom: '#8c8c8c',
  };

  return (
    <div
      style={{
        padding: '12px 16px',
        borderRadius: 8,
        background: '#fff',
        border: `2px solid ${selected ? '#1890ff' : roleColors[agent.role] || '#d9d9d9'}`,
        minWidth: 160,
        boxShadow: selected ? '0 4px 12px rgba(24, 144, 255, 0.3)' : '0 2px 8px rgba(0,0,0,0.1)',
      }}
    >
      <Handle type="target" position={Position.Top} style={{ background: '#1890ff' }} />
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <UserOutlined style={{ color: roleColors[agent.role] || '#8c8c8c', fontSize: 16 }} />
        <div>
          <Text strong style={{ fontSize: 14 }}>{agent.name}</Text>
          <br />
          <Tag color={roleColors[agent.role]} style={{ margin: 0, fontSize: 11 }}>
            {agent.role}
          </Tag>
        </div>
      </div>
      {onDelete && (
        <Tooltip title="删除节点">
          <Button
            type="text"
            size="small"
            danger
            icon={<DeleteOutlined />}
            style={{ position: 'absolute', top: 4, right: 4 }}
            onClick={(e: React.MouseEvent) => {
              e.stopPropagation();
              onDelete(agent.id);
            }}
          />
        </Tooltip>
      )}
      <Handle type="source" position={Position.Bottom} style={{ background: '#52c41a' }} />
    </div>
  );
};

// 节点类型 - 使用 any 避免类型冲突
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodeTypes: any = { agentNode: AgentNode };

interface WorkflowEditorProps {
  agents: AgentConfig[];
  initialTransitions?: Transition[];
  onSave: (transitions: Transition[], agentIds: string[]) => Promise<void>;
  readOnly?: boolean;
}

const WorkflowEditor: React.FC<WorkflowEditorProps> = ({
  agents,
  initialTransitions = [],
  onSave,
  readOnly = false,
}) => {
  // 连接编辑弹窗
  const [edgeModalVisible, setEdgeModalVisible] = useState(false);
  const [currentEdge, setCurrentEdge] = useState<Edge | null>(null);
  const [edgeForm] = Form.useForm();

  // 获取边标签
  const getEdgeLabel = (type: TransitionType): string => {
    switch (type) {
      case 'parallel':
        return '🔀 并行';
      case 'merge':
        return '🔄 汇聚';
      default:
        return '→ 顺序';
    }
  };

  // 获取边颜色
  const getEdgeColor = (type: TransitionType): string => {
    switch (type) {
      case 'parallel':
        return '#52c41a';
      case 'merge':
        return '#722ed1';
      default:
        return '#1890ff';
    }
  };

  // 从transitions生成节点
  const generateNodesFromTransitions = useCallback((transitions: Transition[]) => {
    const agentIds = [...new Set(transitions.flatMap(t => [t.fromAgentId, t.toAgentId]))];
    const nodes: { id: string; type: string; position: { x: number; y: number }; data: { agent: AgentConfig } }[] = [];
    agentIds.forEach((agentId, index) => {
      const agent = agents.find(a => a.id === agentId);
      if (agent) {
        nodes.push({
          id: agentId,
          type: 'agentNode',
          position: { x: 150 + (index % 3) * 220, y: 80 + Math.floor(index / 3) * 150 },
          data: { agent },
        });
      }
    });
    return nodes;
  }, [agents]);

  // 从transitions生成边
  const generateEdgesFromTransitions = useCallback((transitions: Transition[]): Edge[] => {
    return transitions.map((t, index) => ({
      id: `e-${t.fromAgentId}-${t.toAgentId}-${index}`,
      source: t.fromAgentId,
      target: t.toAgentId,
      type: 'smoothstep',
      animated: t.type === 'parallel',
      label: getEdgeLabel(t.type),
      data: { ...t },
      style: { stroke: getEdgeColor(t.type) },
    }));
  }, []);

  // 初始化节点和边
  const initialNodes = useMemo(() => generateNodesFromTransitions(initialTransitions), [initialTransitions, generateNodesFromTransitions]);
  const initialEdges = useMemo(() => generateEdgesFromTransitions(initialTransitions), [initialTransitions, generateEdgesFromTransitions]);

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);
  const [saving, setSaving] = useState(false);

  // 处理连接
  const onConnect = useCallback(
    (params: Connection) => {
      if (readOnly) return;

      // 检查是否已存在相同连接
      const existingEdge = edges.find(
        e => e.source === params.source && e.target === params.target
      );
      if (existingEdge) {
        message.warning('该连接已存在');
        return;
      }

      const newEdge: Edge = {
        id: `e-${params.source}-${params.target}-${Date.now()}`,
        source: params.source!,
        target: params.target!,
        type: 'smoothstep',
        label: '→ 顺序',
        data: {
          fromAgentId: params.source,
          toAgentId: params.target,
          type: 'sequence',
        },
        style: { stroke: '#1890ff' },
      };
      setEdges((eds) => [...eds, newEdge]);
    },
    [edges, readOnly, setEdges]
  );

  // 边点击编辑
  const onEdgeClick = useCallback(
    (_: React.MouseEvent, edge: Edge) => {
      if (readOnly) return;
      setCurrentEdge(edge);
      const edgeData = edge.data as Record<string, unknown> | undefined;
      edgeForm.setFieldsValue({
        type: edgeData?.type || 'sequence',
        trigger: edgeData?.trigger || '',
        condition: edgeData?.condition || '',
        description: edgeData?.description || '',
      });
      setEdgeModalVisible(true);
    },
    [readOnly, edgeForm]
  );

  // 保存边配置
  const handleEdgeSave = async () => {
    const values = await edgeForm.validateFields();
    if (currentEdge) {
      const updatedEdge: Edge = {
        ...currentEdge,
        label: getEdgeLabel(values.type),
        style: { stroke: getEdgeColor(values.type) },
        animated: values.type === 'parallel',
        data: {
          ...currentEdge.data,
          ...values,
          fromAgentId: currentEdge.source,
          toAgentId: currentEdge.target,
        },
      };
      setEdges((eds) => eds.map((e) => (e.id === currentEdge.id ? updatedEdge : e)));
    }
    setEdgeModalVisible(false);
    setCurrentEdge(null);
  };

  // 删除边
  const handleEdgeDelete = useCallback(() => {
    if (currentEdge) {
      setEdges((eds) => eds.filter((e) => e.id !== currentEdge.id));
      setEdgeModalVisible(false);
      setCurrentEdge(null);
    }
  }, [currentEdge, setEdges]);

  // 添加Agent到画布
  const handleAddAgent = useCallback(
    (agentId: string) => {
      if (!agentId) return;
      if (nodes.find((n) => n.id === agentId)) {
        message.info('该Agent已在画布中');
        return;
      }

      const agent = agents.find((a) => a.id === agentId);
      if (!agent) return;

      const newNode = {
        id: agentId,
        type: 'agentNode',
        position: {
          x: 150 + (nodes.length % 3) * 220,
          y: 80 + Math.floor(nodes.length / 3) * 150,
        },
        data: { agent },
      };
      setNodes((nds) => [...nds, newNode]);
    },
    [agents, nodes, setNodes]
  );

  // 删除节点
  const handleDeleteNode = useCallback(
    (nodeId: string) => {
      setNodes((nds) => nds.filter((n) => n.id !== nodeId));
      setEdges((eds) => eds.filter((e) => e.source !== nodeId && e.target !== nodeId));
    },
    [setNodes, setEdges]
  );

  // 更新节点数据
  const updatedNodes = useMemo(() => {
    return nodes.map((node) => ({
      ...node,
      data: {
        ...node.data,
        onDelete: readOnly ? undefined : handleDeleteNode,
      },
    }));
  }, [nodes, readOnly, handleDeleteNode]);

  // 保存团队
  const handleSave = async () => {
    if (nodes.length === 0) {
      message.warning('请先添加Agent节点');
      return;
    }

    const transitions: Transition[] = edges.map((edge) => {
      const edgeData = edge.data as Record<string, unknown> | undefined;
      return {
        fromAgentId: edge.source,
        toAgentId: edge.target,
        type: (edgeData?.type as TransitionType) || 'sequence',
        trigger: edgeData?.trigger as string | undefined,
        condition: edgeData?.condition as string | undefined,
        waitFor: edgeData?.waitFor as string[] | undefined,
        description: edgeData?.description as string | undefined,
      };
    });

    const agentIds = nodes.map((n) => n.id);

    setSaving(true);
    try {
      await onSave(transitions, agentIds);
      message.success('团队保存成功');
    } catch (error: unknown) {
      const err = error as { message?: string };
      message.error(err?.message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  // 可用的Agent列表（未添加到画布的）
  const availableAgents = useMemo(() => {
    const canvasAgentIds = new Set(nodes.map((n) => n.id));
    return agents.filter((a) => !canvasAgentIds.has(a.id));
  }, [agents, nodes]);

  return (
    <div style={{ height: 500, border: '1px solid #d9d9d9', borderRadius: 8, overflow: 'hidden' }}>
      <ReactFlow
        nodes={updatedNodes}
        edges={edges}
        onNodesChange={readOnly ? undefined : onNodesChange}
        onEdgesChange={readOnly ? undefined : onEdgesChange}
        onConnect={onConnect}
        onEdgeClick={onEdgeClick}
        nodeTypes={nodeTypes}
        fitView
        attributionPosition="bottom-left"
      >
        <Controls />
        <MiniMap
          nodeStrokeWidth={3}
          zoomable
          pannable
        />
        <Background variant={BackgroundVariant.Dots} gap={12} size={1} />

        {/* 工具栏 */}
        <Panel position="top-right">
          <Space>
            {!readOnly && availableAgents.length > 0 && (
              <Select
                placeholder="添加Agent到画布"
                style={{ width: 180 }}
                onSelect={(value) => {
                  if (typeof value === 'string') {
                    handleAddAgent(value);
                  }
                }}
                value={undefined}
                showSearch
                filterOption={(input, option) => {
                  const label = option?.label;
                  if (typeof label === 'string') {
                    return label.toLowerCase().includes(input.toLowerCase());
                  }
                  return false;
                }}
                options={availableAgents.map((agent) => ({
                  value: agent.id,
                  label: agent.name,
                }))}
              />
            )}
            {!readOnly && (
              <Button
                type="primary"
                icon={<SaveOutlined />}
                onClick={handleSave}
                loading={saving}
              >
                保存团队
              </Button>
            )}
          </Space>
        </Panel>

        {/* 提示信息 */}
        {nodes.length === 0 && (
          <Panel position="top-center">
            <Card size="small" style={{ background: '#f6ffed', border: '1px solid #b7eb8f' }}>
              <Text type="secondary">
                {!readOnly ? '从右侧下拉框选择Agent添加到画布，然后拖拽连接节点' : '暂无团队配置'}
              </Text>
            </Card>
          </Panel>
        )}
      </ReactFlow>

      {/* 边配置弹窗 */}
      <Modal
        title="配置转换规则"
        open={edgeModalVisible}
        onOk={handleEdgeSave}
        onCancel={() => {
          setEdgeModalVisible(false);
          setCurrentEdge(null);
        }}
        footer={[
          <Popconfirm
            key="delete"
            title="确定删除此连接？"
            onConfirm={handleEdgeDelete}
          >
            <Button danger>删除连接</Button>
          </Popconfirm>,
          <Button key="cancel" onClick={() => setEdgeModalVisible(false)}>
            取消
          </Button>,
          <Button key="submit" type="primary" onClick={handleEdgeSave}>
            确定
          </Button>,
        ]}
      >
        <Form form={edgeForm} layout="vertical">
          <Form.Item name="type" label="转换类型" rules={[{ required: true }]}>
            <Select>
              <Option value="sequence">
                <Space>
                  <ArrowRightOutlined style={{ color: '#1890ff' }} />
                  <span>顺序执行</span>
                  <Text type="secondary">(一个完成后触发下一个)</Text>
                </Space>
              </Option>
              <Option value="parallel">
                <Space>
                  <BranchesOutlined style={{ color: '#52c41a' }} />
                  <span>并行触发</span>
                  <Text type="secondary">(同时触发多个Agent)</Text>
                </Space>
              </Option>
              <Option value="merge">
                <Space>
                  <MergeCellsOutlined style={{ color: '#722ed1' }} />
                  <span>汇聚等待</span>
                  <Text type="secondary">(等待多个Agent都完成)</Text>
                </Space>
              </Option>
            </Select>
          </Form.Item>

          <Form.Item name="trigger" label="触发信号">
            <Input placeholder="例如：需求分析完成" />
          </Form.Item>

          <Form.Item name="condition" label="条件表达式">
            <Input placeholder="例如：contains:关键词 或 regex:正则" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <TextArea rows={2} placeholder="描述这个转换的作用" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default WorkflowEditor;