import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Modal, Form, Input, Switch, Tag, message, Popconfirm, Typography, Spin
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, ShopOutlined, RocketOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Market, AddMarketRequest } from '@/types';

const { Text } = Typography;

// Cron 表达式校验（简化版：支持 5 位标准 cron）
const validateCron = (cron: string): boolean => {
  if (!cron) return true; // 空值允许（使用默认）
  const parts = cron.trim().split(/\s+/);
  if (parts.length !== 5) return false;

  // 每部分的校验规则
  const validatePart = (part: string, min: number, max: number): boolean => {
    if (part === '*') return true;
    if (part.includes('/')) {
      const [base, step] = part.split('/');
      if (base !== '*' && !/^\d+$/.test(base)) return false;
      if (!/^\d+$/.test(step)) return false;
      return true;
    }
    if (part.includes('-')) {
      const [start, end] = part.split('-');
      if (!/^\d+$/.test(start) || !/^\d+$/.test(end)) return false;
      const s = parseInt(start), e = parseInt(end);
      return s >= min && s <= max && e >= min && e <= max && s <= e;
    }
    if (part.includes(',')) {
      const values = part.split(',');
      return values.every(v => /^\d+$/.test(v) && parseInt(v) >= min && parseInt(v) <= max);
    }
    if (!/^\d+$/.test(part)) return false;
    const val = parseInt(part);
    return val >= min && val <= max;
  };

  // 分 时 日 月 周
  return validatePart(parts[0], 0, 59) &&
         validatePart(parts[1], 0, 23) &&
         validatePart(parts[2], 1, 31) &&
         validatePart(parts[3], 1, 12) &&
         validatePart(parts[4], 0, 6);
};

// Cron 表达式示例说明
const cronExamples = [
  '0 0 * * * - 每天 0:00',
  '0 12 * * * - 每天 12:00',
  '0 0 * * 1 - 每周一 0:00',
  '0 0 1 * * - 每月 1 号 0:00',
  '*/30 * * * * - 每 30 分钟',
];

const MarketManagement: React.FC = () => {
  const [markets, setMarkets] = useState<Market[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingMarket, setEditingMarket] = useState<Market | null>(null);
  const [cronModalVisible, setCronModalVisible] = useState(false);
  const [editingCronMarket, setEditingCronMarket] = useState<Market | null>(null);
  const [cronForm] = Form.useForm();
  const [form] = Form.useForm();
  // 默认市场相关状态
  const [defaultMarketConfig, setDefaultMarketConfig] = useState<{ name: string; url: string; branch: string } | null>(null);
  const [showAddDefaultButton, setShowAddDefaultButton] = useState(false);
  const [addingDefaultMarket, setAddingDefaultMarket] = useState(false);

  useEffect(() => {
    loadMarkets();
    loadDefaultMarketConfig();
  }, []);

  const loadDefaultMarketConfig = async () => {
    try {
      const config = await api.markets.getDefaultConfig();
      setDefaultMarketConfig(config);
    } catch (error: any) {
      // 获取默认配置失败，不影响主流程
      console.warn('获取默认市场配置失败:', error);
    }
  };

  const loadMarkets = async () => {
    setLoading(true);
    try {
      const result = await api.markets.list();
      setMarkets(result.data);
    } catch (error: any) {
      message.error(error.message || '加载市场列表失败');
    } finally {
      setLoading(false);
    }
  };

  // 检查是否需要显示添加默认市场按钮
  useEffect(() => {
    if (defaultMarketConfig && defaultMarketConfig.url) {
      // 检查是否已存在该URL的市场
      const exists = markets.some(m => m.url === defaultMarketConfig.url);
      setShowAddDefaultButton(!exists);
    } else {
      setShowAddDefaultButton(false);
    }
  }, [markets, defaultMarketConfig]);

  const handleAddDefaultMarket = async () => {
    setAddingDefaultMarket(true);
    try {
      await api.markets.addDefaultMarket();
      message.success('已添加默认市场');
      loadMarkets();
    } catch (error: any) {
      message.error(error.message || '添加默认市场失败');
    } finally {
      setAddingDefaultMarket(false);
    }
  };

  const handleAdd = () => {
    setEditingMarket(null);
    form.resetFields();
    form.setFieldsValue({ branch: 'main' });
    setModalVisible(true);
  };

  const handleEdit = (market: Market) => {
    setEditingMarket(market);
    form.setFieldsValue({
      name: market.name,
      url: market.url,
      branch: market.branch,
    });
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.markets.delete(id);
      message.success('市场已删除');
      loadMarkets();
    } catch (error: any) {
      message.error(error.message || '删除失败');
    }
  };

  const handleToggleEnabled = async (market: Market, enabled: boolean) => {
    try {
      // 禁用时自动关闭自动更新
      if (!enabled && market.autoUpdate) {
        await api.markets.update(market.id, { enabled, autoUpdate: false });
        message.success('市场已禁用，自动更新已关闭');
      } else {
        await api.markets.update(market.id, { enabled });
        message.success(enabled ? '市场已启用' : '市场已禁用');
      }
      loadMarkets();
    } catch (error: any) {
      message.error(error.message || '操作失败');
    }
  };

  const handleToggleAutoUpdate = async (market: Market, autoUpdate: boolean) => {
    try {
      await api.markets.update(market.id, { autoUpdate });
      message.success(autoUpdate ? '已开启自动更新' : '已关闭自动更新');
      loadMarkets();
    } catch (error: any) {
      message.error(error.message || '操作失败');
    }
  };

  const handleEditCron = (market: Market) => {
    setEditingCronMarket(market);
    cronForm.setFieldsValue({ checkInterval: market.checkInterval || '0 0 * * *' });
    setCronModalVisible(true);
  };

  const handleCronSubmit = async () => {
    try {
      const values = await cronForm.validateFields();
      const cron = values.checkInterval?.trim() || '0 0 * * *';

      if (!validateCron(cron)) {
        message.error('Cron 表达式格式错误');
        return;
      }

      await api.markets.update(editingCronMarket!.id, { checkInterval: cron });
      message.success('更新间隔已设置');
      setCronModalVisible(false);
      loadMarkets();
    } catch (error: any) {
      message.error(error.message || '操作失败');
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      if (editingMarket) {
        await api.markets.update(editingMarket.id, {
          name: values.name,
          url: values.url,
          branch: values.branch,
        });
        message.success('市场已更新');
      } else {
        const req: AddMarketRequest = {
          name: values.name,
          url: values.url,
          branch: values.branch || 'main',
        };
        await api.markets.add(req);
        message.success('市场已添加');
      }
      setModalVisible(false);
      loadMarkets();
    } catch (error: any) {
      message.error(error.message || '操作失败');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'URL',
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
      width: 300,
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'green' : 'default'}>
          {enabled ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '自动更新',
      dataIndex: 'autoUpdate',
      key: 'autoUpdate',
      render: (autoUpdate: boolean, record: Market) => (
        <Space>
          <Switch
            size="small"
            checked={autoUpdate}
            onChange={(checked) => handleToggleAutoUpdate(record, checked)}
            disabled={!record.enabled}
          />
          <Button
            type="link"
            size="small"
            onClick={() => handleEditCron(record)}
            style={{ padding: 0, fontSize: 12 }}
            disabled={!record.enabled}
          >
            {record.checkInterval || '0 0 * * *'}
          </Button>
        </Space>
      ),
    },
    {
      title: '最后同步',
      dataIndex: 'lastSyncedAt',
      key: 'lastSyncedAt',
      render: (time?: string) => time ? new Date(time).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      render: (_: any, record: Market) => (
        <Space size="small">
          <Space size={4}>
            <Switch
              size="small"
              checked={record.enabled}
              onChange={(checked) => handleToggleEnabled(record, checked)}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>
              {record.enabled ? '启用' : '禁用'}
            </Text>
          </Space>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          />
          <Popconfirm
            title="确定删除此市场？"
            onConfirm={() => handleDelete(record.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="market-management">
      <Card
        title={
          <Space>
            <ShopOutlined />
            <span>市场管理</span>
          </Space>
        }
        extra={
          <Space>
            {showAddDefaultButton && defaultMarketConfig && (
              <Button
                type="default"
                icon={<RocketOutlined />}
                onClick={handleAddDefaultMarket}
                loading={addingDefaultMarket}
              >
                添加默认市场
              </Button>
            )}
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
              添加市场
            </Button>
          </Space>
        }
      >
        <Spin spinning={loading}>
          <Table
            dataSource={markets}
            columns={columns}
            rowKey="id"
            pagination={false}
          />
        </Spin>
      </Card>

      <Modal
        title={editingMarket ? '编辑市场' : '添加市场'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="市场名称"
            rules={[{ required: true, message: '请输入市场名称' }]}
          >
            <Input placeholder="如：Colink官方市场" />
          </Form.Item>
          <Form.Item
            name="url"
            label="Git仓库URL"
            rules={[{ required: true, message: '请输入Git仓库URL' }]}
          >
            <Input placeholder="https://gitee.com/xxx/marketplace.git" />
          </Form.Item>
          <Form.Item
            name="branch"
            label="分支"
          >
            <Input placeholder="main" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="设置更新间隔"
        open={cronModalVisible}
        onOk={handleCronSubmit}
        onCancel={() => setCronModalVisible(false)}
        width={500}
      >
        <Form form={cronForm} layout="vertical">
          <Form.Item
            name="checkInterval"
            label="Cron 表达式"
            rules={[
              { required: false },
              {
                validator: (_, value) => {
                  if (!value || validateCron(value)) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('格式错误：需要 5 位标准 cron（分 时 日 月 周）'));
                },
              },
            ]}
            extra={
              <div style={{ marginTop: 8 }}>
                <Text type="secondary">格式：分 时 日 月 周（5 位）</Text>
                <br />
                <Text type="secondary">示例：</Text>
                <ul style={{ margin: '4px 0', padding: '0 16px', listStyle: 'disc' }}>
                  {cronExamples.map(ex => (
                    <li key={ex}><Text type="secondary" style={{ fontSize: 12 }}>{ex}</Text></li>
                  ))}
                </ul>
              </div>
            }
          >
            <Input placeholder="0 0 * * *" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default MarketManagement;