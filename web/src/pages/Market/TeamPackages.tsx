import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Tag, message, Popconfirm, Spin
} from 'antd';
import {
  CloudDownloadOutlined, SyncOutlined, ShopOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { MarketPackage } from '@/types';

const TeamPackages: React.FC = () => {
  const [packages, setPackages] = useState<MarketPackage[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshingAll, setRefreshingAll] = useState(false);
  const [syncingPackage, setSyncingPackage] = useState<string | null>(null);

  useEffect(() => {
    loadPackages();
  }, []);

  const loadPackages = async () => {
    setLoading(true);
    try {
      const result = await api.markets.getTeamPackages();
      setPackages(result.data);
    } catch (error: any) {
      message.error(error.response?.data?.error || '加载团队包列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleRefreshAll = async () => {
    setRefreshingAll(true);
    try {
      const markets = await api.markets.list();
      for (const market of markets.data) {
        if (market.enabled) {
          await api.markets.refresh(market.id);
        }
      }
      message.success('所有市场已刷新');
      loadPackages();
    } catch (error: any) {
      message.error('刷新失败');
    } finally {
      setRefreshingAll(false);
    }
  };

  const handleSync = async (pkg: MarketPackage) => {
    setSyncingPackage(pkg.name);
    try {
      await api.teamPackages.syncPackage(pkg.name, undefined, pkg.marketId);
      message.success(`团队包 ${pkg.name} 导入成功`);
      loadPackages();
    } catch (error: any) {
      message.error(error.response?.data?.error || '导入失败');
    } finally {
      setSyncingPackage(null);
    }
  };

  const getStatusTag = (status: string) => {
    const colors: Record<string, string> = {
      new: 'blue',
      update: 'orange',
      latest: 'green',
    };
    const labels: Record<string, string> = {
      new: '未导入',
      update: '待更新',
      latest: '已导入',
    };
    return <Tag color={colors[status]}>{labels[status]}</Tag>;
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 120,
    },
    {
      title: '来源市场',
      dataIndex: 'marketName',
      key: 'marketName',
      width: 150,
    },
    {
      title: '本地版本',
      dataIndex: 'localVersion',
      key: 'localVersion',
      width: 120,
      render: (v?: string) => v || '-',
    },
    {
      title: '状态',
      dataIndex: 'localStatus',
      key: 'localStatus',
      width: 100,
      render: getStatusTag,
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_: any, record: MarketPackage) => {
        const isSyncing = syncingPackage === record.name;
        const buttonText = record.localStatus === 'new' ? '导入' :
                           record.localStatus === 'update' ? '更新' : '重新导入';
        return (
          <Popconfirm
            title={`确定要${buttonText}团队包 "${record.name}" 吗？`}
            onConfirm={() => handleSync(record)}
          >
            <Button
              type={record.localStatus === 'new' ? 'primary' : 'default'}
              size="small"
              icon={<CloudDownloadOutlined />}
              loading={isSyncing}
            >
              {buttonText}
            </Button>
          </Popconfirm>
        );
      },
    },
  ];

  return (
    <div className="team-packages">
      <Card
        title={
          <Space>
            <ShopOutlined />
            <span>远程团队包</span>
          </Space>
        }
        extra={
          <Button icon={<SyncOutlined />} onClick={handleRefreshAll} loading={refreshingAll}>
            刷新全部市场
          </Button>
        }
      >
        <Spin spinning={loading}>
          <Table
            dataSource={packages}
            columns={columns}
            rowKey={(record) => `${record.marketId}-${record.name}`}
            pagination={{ pageSize: 20 }}
          />
        </Spin>
      </Card>
    </div>
  );
};

export default TeamPackages;