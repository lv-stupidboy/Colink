import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Modal, Form, Input, Switch, Tag, message, Popconfirm, Typography, Spin
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SyncOutlined, ShopOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Market, AddMarketRequest } from '@/types';

const { Text } = Typography;

const MarketManagement: React.FC = () => {
  const [markets, setMarkets] = useState<Market[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingMarket, setEditingMarket] = useState<Market | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadMarkets();
  }, []);

  const loadMarkets = async () => {
    setLoading(true);
    try {
      const result = await api.markets.list();
      setMarkets(result.data);
    } catch (error: any) {
      message.error(error.response?.data?.error || '加载市场列表失败');
    } finally {
      setLoading(false);
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
      message.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleRefresh = async (id: string) => {
    try {
      const result = await api.markets.refresh(id);
      message.success(`市场刷新成功，解析到 ${result.plugins} 个插件`);
      loadMarkets();
    } catch (error: any) {
      message.error(error.response?.data?.error || '刷新失败');
    }
  };

  const handleToggleEnabled = async (market: Market, enabled: boolean) => {
    try {
      await api.markets.update(market.id, { enabled });
      message.success(enabled ? '市场已启用' : '市场已禁用');
      loadMarkets();
    } catch (error: any) {
      message.error('操作失败');
    }
  };

  const handleToggleAutoUpdate = async (market: Market, autoUpdate: boolean) => {
    try {
      await api.markets.update(market.id, { autoUpdate });
      message.success(autoUpdate ? '已开启自动更新' : '已关闭自动更新');
      loadMarkets();
    } catch (error: any) {
      message.error('操作失败');
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      if (editingMarket) {
        await api.markets.update(editingMarket.id, { name: values.name });
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
      message.error(error.response?.data?.error || '操作失败');
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
          <Text type="secondary">{record.checkInterval}</Text>
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
      width: 180,
      render: (_: any, record: Market) => (
        <Space size="small">
          <Switch
            size="small"
            checked={record.enabled}
            onChange={(checked) => handleToggleEnabled(record, checked)}
          />
          <Button
            size="small"
            icon={<SyncOutlined />}
            onClick={() => handleRefresh(record.id)}
            disabled={!record.enabled}
          >
            刷新
          </Button>
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
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加市场
          </Button>
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
            <Input placeholder="https://gitee.com/xxx/marketplace.git" disabled={!!editingMarket} />
          </Form.Item>
          <Form.Item
            name="branch"
            label="分支"
          >
            <Input placeholder="main" disabled={!!editingMarket} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default MarketManagement;