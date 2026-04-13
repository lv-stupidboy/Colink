import React, { useState, useEffect } from 'react';
import { Card, Form, Switch, Select, Typography, Space, Button, message, Alert, Tag } from 'antd';
import {
  SettingOutlined,
  ApiOutlined,
  BellOutlined,
  UserOutlined,
  SaveOutlined,
  AlertOutlined,
  DesktopOutlined,
} from '@ant-design/icons';
import { useAppStore } from '@/store';
import {
  isNotificationSupported,
  requestNotificationPermission,
  getNotificationPermission,
} from '@/utils/systemNotification';

const { Title, Text } = Typography;

const GeneralSettings: React.FC = () => {
  const [form] = Form.useForm();

  // 阻塞提醒开关状态
  const [reminderEnabled, setReminderEnabled] = useState(true);

  // 系统通知权限状态
  const [notificationPermission, setNotificationPermission] = useState<'granted' | 'denied' | 'default' | 'unsupported'>('default');

  // 从 Store 获取阻塞提醒相关 actions
  const setBlockingReminderEnabled = useAppStore((state) => state.setBlockingReminderEnabled);

  // 初始化时从 localStorage 读取阻塞提醒开关状态，并检查系统通知权限
  useEffect(() => {
    const stored = localStorage.getItem('isdp_blocking_reminder_enabled');
    setReminderEnabled(stored !== 'false');  // 默认 true

    // 检查系统通知权限状态
    if (isNotificationSupported()) {
      setNotificationPermission(getNotificationPermission());
    } else {
      setNotificationPermission('unsupported');
    }
  }, []);

  // 实时保存阻塞提醒开关状态
  const handleReminderChange = (checked: boolean) => {
    setReminderEnabled(checked);
    setBlockingReminderEnabled(checked);
    message.success(checked ? '已开启阻塞提醒' : '已关闭阻塞提醒');
  };

  // 请求系统通知权限
  const handleRequestNotificationPermission = async () => {
    const granted = await requestNotificationPermission();
    if (granted) {
      setNotificationPermission('granted');
      message.success('系统通知权限已授权');
    } else {
      setNotificationPermission('denied');
      message.warning('系统通知权限被拒绝，请检查浏览器设置');
    }
  };

  const handleSave = async (values: any) => {
    console.log('Settings saved:', values);
    message.success('设置已保存');
  };

  return (
    <div className="general-settings">
      <div style={{ marginBottom: 24 }}>
        <Title level={3}>
          <Space>
            <SettingOutlined />
            通用设置
          </Space>
        </Title>
        <Text type="secondary">配置平台参数和个性化选项</Text>
      </div>

      <Card
        title={
          <Space>
            <ApiOutlined />
            API 配置
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSave}
          initialValues={{
            mcpEnabled: true,
            sandboxEnabled: true,
            logLevel: 'info',
          }}
        >
          <Form.Item
            name="mcpEnabled"
            label="MCP 协议"
            valuePropName="checked"
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          <Form.Item
            name="sandboxEnabled"
            label="沙箱环境"
            valuePropName="checked"
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          <Form.Item
            name="logLevel"
            label="日志级别"
          >
            <Select>
              <Select.Option value="debug">Debug</Select.Option>
              <Select.Option value="info">Info</Select.Option>
              <Select.Option value="warn">Warn</Select.Option>
              <Select.Option value="error">Error</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<SaveOutlined />}>
              保存配置
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {/* 系统消息提醒设置卡片 */}
      <Card
        title={
          <Space>
            <AlertOutlined />
            系统消息提醒设置
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        <Form layout="vertical">
          <Form.Item
            label="Agent 完成提醒"
            tooltip="当 Agent 执行完成时，发送系统级通知提醒（即使切换到其他应用也能收到）"
          >
            <Space direction="vertical">
              <Space>
                <Switch
                  checked={reminderEnabled}
                  onChange={handleReminderChange}
                  checkedChildren="开启"
                  unCheckedChildren="关闭"
                />
                <Text type="secondary">
                  {reminderEnabled ? 'Agent 完成时发送系统通知' : '已关闭，无提醒'}
                </Text>
              </Space>
              <Text type="secondary" style={{ fontSize: 12, marginTop: 8, display: 'block' }}>
                提醒场景：Agent 执行完成、任务调度结束等待用户指示
              </Text>
              <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
                支持累积提醒：多个 Agent 完成时会汇总显示通知数量
              </Text>
            </Space>
          </Form.Item>
        </Form>
      </Card>

      <Card
        title={
          <Space>
            <BellOutlined />
            通知设置
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        {/* 系统通知权限状态 */}
        <div style={{ marginBottom: 16 }}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Space>
              <DesktopOutlined />
              <Text strong>系统通知权限</Text>
              {notificationPermission === 'granted' && <Tag color="green">已授权</Tag>}
              {notificationPermission === 'denied' && <Tag color="red">已拒绝</Tag>}
              {notificationPermission === 'default' && <Tag color="orange">未请求</Tag>}
              {notificationPermission === 'unsupported' && <Tag color="default">不支持</Tag>}
            </Space>

            {notificationPermission === 'unsupported' && (
              <Alert
                type="warning"
                message="当前浏览器不支持系统通知功能"
                showIcon
              />
            )}

            {notificationPermission === 'denied' && (
              <Alert
                type="error"
                message="系统通知权限已被拒绝"
                description="请在浏览器设置中允许通知权限，以便在其他应用时也能收到 Agent 完成提醒"
                showIcon
              />
            )}

            {notificationPermission === 'default' && (
              <>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  开启系统通知后，即使切换到其他应用也能收到 Agent 完成提醒
                </Text>
                <Button
                  type="primary"
                  onClick={handleRequestNotificationPermission}
                  icon={<DesktopOutlined />}
                >
                  授权系统通知
                </Button>
              </>
            )}

            {notificationPermission === 'granted' && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                ✓ 已开启系统通知，Agent 完成时会发送系统级提醒
              </Text>
            )}
          </Space>
        </div>

        <Form
          layout="vertical"
          initialValues={{
            emailNotification: false,
            soundNotification: true,
          }}
        >
          <Form.Item
            name="soundNotification"
            label="声音提示"
            valuePropName="checked"
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        </Form>
      </Card>

      <Card
        title={
          <Space>
            <UserOutlined />
            个性化
          </Space>
        }
      >
        <Form
          layout="vertical"
          initialValues={{
            theme: 'light',
            language: 'zh-CN',
          }}
        >
          <Form.Item name="theme" label="主题">
            <Select>
              <Select.Option value="light">浅色</Select.Option>
              <Select.Option value="dark">深色</Select.Option>
              <Select.Option value="auto">跟随系统</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item name="language" label="语言">
            <Select>
              <Select.Option value="zh-CN">简体中文</Select.Option>
              <Select.Option value="en-US">English</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default GeneralSettings;