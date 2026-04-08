import React from 'react';
import { Card, Form, Switch, Select, Typography, Space, Button, message } from 'antd';
import {
  SettingOutlined,
  ApiOutlined,
  BellOutlined,
  UserOutlined,
  SaveOutlined,
} from '@ant-design/icons';

const { Title, Text } = Typography;

const GeneralSettings: React.FC = () => {
  const [form] = Form.useForm();

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

      <Card
        title={
          <Space>
            <BellOutlined />
            通知设置
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        <Form
          layout="vertical"
          initialValues={{
            emailNotification: false,
            soundNotification: true,
            desktopNotification: true,
          }}
        >
          <Form.Item
            name="emailNotification"
            label="邮件通知"
            valuePropName="checked"
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          <Form.Item
            name="soundNotification"
            label="声音提示"
            valuePropName="checked"
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          <Form.Item
            name="desktopNotification"
            label="桌面通知"
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