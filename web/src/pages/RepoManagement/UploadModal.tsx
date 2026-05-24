import React, { useEffect, useState } from 'react';
import { Modal, Form, Input, Upload, message, Space, Button, Typography } from 'antd';
import api from '@/api/client';
import PathSelector from '@/components/PathSelector';
import type { RuntimeConfig } from '@/types';

const { Text } = Typography;

interface UploadModalProps {
  visible: boolean;
  onSuccess: () => void;
  onCancel: () => void;
}

const UploadModal: React.FC<UploadModalProps> = ({ visible, onSuccess, onCancel }) => {
  const [form] = Form.useForm();
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [fileList, setFileList] = useState<any[]>([]);
  const [pathSelectorVisible, setPathSelectorVisible] = useState(false);
  const [runtimeConfig, setRuntimeConfig] = useState<RuntimeConfig | null>(null);

  useEffect(() => {
    if (visible) {
      form.resetFields();
      setFileList([]);
      loadRuntimeConfig();
    }
  }, [visible]);

  const loadRuntimeConfig = async () => {
    try {
      const config = await api.runtime.config();
      setRuntimeConfig(config);
      form.setFieldsValue({ targetPath: config.defaultPath || config.workspacePath });
    } catch (error) {
      console.error('加载运行配置失败', error);
    }
  };

  const handleSubmit = async (values: any) => {
    if (fileList.length === 0) {
      message.warning('请选择 ZIP 文件');
      return;
    }
    setConfirmLoading(true);
    try {
      const file = fileList[0].originFileObj || fileList[0];
      await api.repos.upload(file, values.targetPath, values.name || undefined);
      message.success('上传并解压成功');
      onSuccess();
    } catch (error: any) {
      message.error(error?.response?.data?.error || error?.message || '上传失败');
    } finally {
      setConfirmLoading(false);
    }
  };

  const handleFileChange = (info: any) => {
    setFileList(info.fileList.slice(-1));
    if (info.fileList.length > 0) {
      const fileName = info.fileList[0].name;
      const nameWithoutExt = fileName.replace(/\.zip$/i, '');
      form.setFieldsValue({ name: nameWithoutExt });
    }
  };

  return (
    <>
      <Modal
        title="导入本地文件"
        open={visible}
        onOk={() => form.submit()}
        onCancel={onCancel}
        confirmLoading={confirmLoading}
        okText="上传并解压"
        width={500}
      >
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item name="name" label="仓库名称">
            <Input placeholder="可选，自动从 ZIP 文件名填充" autoComplete="off" />
          </Form.Item>
          <Form.Item label="ZIP文件" required>
            <Upload.Dragger
              accept=".zip"
              maxCount={1}
              fileList={fileList}
              beforeUpload={() => false}
              onChange={handleFileChange}
            >
              <p style={{ color: 'var(--text-secondary)' }}>点击或拖拽 ZIP 文件到此区域</p>
            </Upload.Dragger>
          </Form.Item>
          <Form.Item label="目标路径" required>
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="targetPath" noStyle rules={[{ required: true, message: '请选择目标路径' }]}>
                <Input placeholder="输入或选择解压目标路径" autoComplete="off" />
              </Form.Item>
              <Button onClick={() => setPathSelectorVisible(true)}>浏览</Button>
            </Space.Compact>
          </Form.Item>
          <Text type="secondary">若不设则存到工作空间/{"{仓库名称}"}</Text>
        </Form>
      </Modal>
      <PathSelector
        visible={pathSelectorVisible}
        title="选择解压目标路径"
        placeholder="输入或选择解压目标路径..."
        browseApi={api.repos.browse}
        createFolderApi={api.repos.createFolder}
        initialPath={form.getFieldValue('targetPath') || runtimeConfig?.defaultPath || runtimeConfig?.workspacePath || ''}
        onSelect={(path) => {
          form.setFieldsValue({ targetPath: path });
          setPathSelectorVisible(false);
        }}
        onCancel={() => setPathSelectorVisible(false)}
      />
    </>
  );
};

export default UploadModal;
