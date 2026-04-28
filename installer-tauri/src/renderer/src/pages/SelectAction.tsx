import React from 'react';
import { Button, Card, Typography, Space, Popconfirm } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';

const { Title, Text } = Typography;

interface SelectActionProps {
  installDir: string;
  version?: string;
  onUpgrade: () => void;
  onReinstall: () => void;
  onUninstall: () => void;
  onCancel: () => void;
}

const SelectAction: React.FC<SelectActionProps> = ({
  installDir,
  version,
  onUpgrade,
  onReinstall,
  onUninstall,
  onCancel,
}) => {
  return (
    <div style={{
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      textAlign: 'center'
    }}>
      <Title level={3} style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>Colink 已安装</Title>

      <Text type="secondary" style={{ marginBottom: 24, fontSize: 14 }}>
        检测到 Colink 已安装。请选择下一步操作。
      </Text>

      <Text type="secondary" style={{ marginBottom: 32, fontSize: 13 }}>
        安装位置：<code style={{ background: '#f5f5f5', padding: '2px 8px', borderRadius: 4 }}>{installDir}</code>
        {version && <span style={{ marginLeft: 16 }}>版本：{version}</span>}
      </Text>

      <Space direction="vertical" size="large" style={{ width: '100%', maxWidth: 400 }}>
        <Card
          hoverable
          onClick={onUpgrade}
          style={{ cursor: 'pointer', borderColor: '#52c41a' }}
        >
          <Title level={4} style={{ marginBottom: 8, color: '#52c41a' }}>升级</Title>
          <Text type="secondary">升级到新版本，保留现有数据和配置</Text>
        </Card>

        <Card
          hoverable
          onClick={onReinstall}
          style={{ cursor: 'pointer', borderColor: '#faad14' }}
        >
          <Title level={4} style={{ marginBottom: 8, color: '#faad14' }}>重新安装</Title>
          <Text type="secondary">更改安装位置或覆盖安装（可选保留数据）</Text>
        </Card>

        <Popconfirm
          title="确认卸载"
          description="卸载将删除程序文件，可选择保留用户数据。是否继续？"
          onConfirm={onUninstall}
          okText="确认卸载"
          cancelText="取消"
          okButtonProps={{ danger: true }}
        >
          <Card
            hoverable
            style={{ cursor: 'pointer', borderColor: '#ff4d4f' }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <DeleteOutlined style={{ color: '#ff4d4f', marginRight: 8 }} />
              <Title level={4} style={{ marginBottom: 0, color: '#ff4d4f' }}>卸载</Title>
            </div>
            <Text type="secondary" style={{ marginTop: 8, display: 'block' }}>移除 Colink 程序</Text>
          </Card>
        </Popconfirm>

        <Button block size="large" onClick={onCancel}>
          取消
        </Button>
      </Space>
    </div>
  );
};

export default SelectAction;