import React, { useState, useEffect } from 'react';
import { Card, Form, Switch, Typography, Space, Button, message, Alert, Tag } from 'antd';
import {
  SettingOutlined,
  AlertOutlined,
  DesktopOutlined,
  SoundOutlined,
} from '@ant-design/icons';
import { useAppStore } from '@/store';
import {
  isNotificationSupported,
  requestNotificationPermission,
  getNotificationPermission,
  isNotificationSoundEnabled,
  setNotificationSoundEnabled,
  playNotificationSound,
} from '@/utils/systemNotification';

const { Title, Text } = Typography;

const GeneralSettings: React.FC = () => {
  // 阻塞提醒开关状态
  const [reminderEnabled, setReminderEnabled] = useState(true);

  // 提示音开关状态
  const [soundEnabled, setSoundEnabled] = useState(true);

  // 系统通知权限状态
  const [notificationPermission, setNotificationPermission] = useState<'granted' | 'denied' | 'default' | 'unsupported'>('default');

  // 从 Store 获取阻塞提醒相关 actions
  const setBlockingReminderEnabled = useAppStore((state) => state.setBlockingReminderEnabled);

  // 初始化时从 localStorage 读取状态，并检查系统通知权限
  useEffect(() => {
    const stored = localStorage.getItem('isdp_blocking_reminder_enabled');
    setReminderEnabled(stored !== 'false');  // 默认 true

    // 检查提示音开关状态
    setSoundEnabled(isNotificationSoundEnabled());

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

  // 提示音开关变化
  const handleSoundChange = (checked: boolean) => {
    setSoundEnabled(checked);
    setNotificationSoundEnabled(checked);
    message.success(checked ? '已开启提示音' : '已关闭提示音');
  };

  // 测试提示音
  const handleTestSound = async () => {
    try {
      await playNotificationSound();
      message.info('提示音已播放');
    } catch (error) {
      message.error('播放失败，请检查浏览器设置');
    }
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

          {/* 提示音开关 */}
          <Form.Item
            label="提示音"
            tooltip="系统通知时播放提示音"
          >
            <Space direction="vertical">
              <Space>
                <Switch
                  checked={soundEnabled}
                  onChange={handleSoundChange}
                  checkedChildren="开启"
                  unCheckedChildren="关闭"
                />
                <Text type="secondary">
                  {soundEnabled ? '通知时播放提示音' : '静音模式'}
                </Text>
                {soundEnabled && (
                  <Button
                    size="small"
                    icon={<SoundOutlined />}
                    onClick={handleTestSound}
                  >
                    测试
                  </Button>
                )}
              </Space>
            </Space>
          </Form.Item>

          {/* 系统通知权限状态 */}
          <Form.Item label="系统通知权限">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space>
                <DesktopOutlined />
                <Text strong>权限状态</Text>
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
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default GeneralSettings;