import React, { useEffect, useState } from 'react';
import { Modal, Form, Input, Select, Button, message, Space, Typography } from 'antd';
import api from '@/api/client';
import PathSelector from '@/components/PathSelector';
import type { RuntimeConfig } from '@/types';
import type { RemoteBranch } from '@/types/localRepo';

const { Text } = Typography;

interface CloneModalProps {
  visible: boolean;
  onSuccess: () => void;
  onCancel: () => void;
}

interface BranchOptionGroup {
  label: string;
  options: { label: string; value: string }[];
}

const sshGitUrlPattern = /^(ssh:\/\/[^\s@]+@[^\s/]+\/.+|[^\s/@]+@[^\s:]+:.+)$/;

const CloneModal: React.FC<CloneModalProps> = ({ visible, onSuccess, onCancel }) => {
  const [form] = Form.useForm();
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [branchLoading, setBranchLoading] = useState(false);
  const [branchOptions, setBranchOptions] = useState<BranchOptionGroup[]>([]);
  const [pathSelectorVisible, setPathSelectorVisible] = useState(false);
  const [runtimeConfig, setRuntimeConfig] = useState<RuntimeConfig | null>(null);

  useEffect(() => {
    if (visible) {
      form.resetFields();
      setBranchOptions([]);
      loadRuntimeConfig();
    }
  }, [visible]);

  const loadRuntimeConfig = async () => {
    try {
      const config = await api.runtime.config();
      setRuntimeConfig(config);
    } catch (error) {
      console.error('加载运行配置失败', error);
    }
  };

  const handleFetchBranches = async () => {
    try {
      await form.validateFields(['gitUrl']);
    } catch {
      return;
    }

    const gitUrl = form.getFieldValue('gitUrl');
    setBranchLoading(true);
    try {
      const branches: RemoteBranch[] = await api.repos.remoteBranches(gitUrl);
      const branchItems = branches.filter(b => b.type === 'branch');
      const tagItems = branches.filter(b => b.type === 'tag');
      const groups: BranchOptionGroup[] = [];
      if (branchItems.length > 0) {
        groups.push({
          label: '分支',
          options: branchItems.map(b => ({ label: b.name, value: b.name })),
        });
      }
      if (tagItems.length > 0) {
        groups.push({
          label: 'Tag',
          options: tagItems.map(b => ({ label: b.name, value: b.name })),
        });
      }
      setBranchOptions(groups);
      if (branchItems.length > 0) {
        form.setFieldsValue({ branch: branchItems[0].name });
      }
    } catch (error: any) {
      message.error(error?.response?.data?.error || error?.message || '获取分支失败');
    } finally {
      setBranchLoading(false);
    }
  };

  const handleGitUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const url = e.target.value;
    const match = url.match(/[/:]([^/:]+?)(?:\.git)?$/);
    if (match) {
      form.setFieldsValue({ name: match[1] });
    }
    setBranchOptions([]);
    form.setFieldsValue({ branch: undefined });
  };

  const getTargetPathPlaceholder = () => {
    const configuredPath = runtimeConfig?.workspacePath || runtimeConfig?.defaultPath;
    if (configuredPath) {
      return configuredPath;
    }
    return '用户根目录';
  };

  const handleSubmit = async (values: any) => {
    setConfirmLoading(true);
    try {
      await api.repos.clone({
        gitUrl: values.gitUrl,
        branch: values.branch,
        name: values.name,
        targetPath: values.targetPath,
      });
      message.success('拉取成功');
      onSuccess();
    } catch (error: any) {
      message.error(error?.response?.data?.error || error?.message || '拉取失败');
    } finally {
      setConfirmLoading(false);
    }
  };

  return (
    <>
      <Modal
        title="远程代码仓拉取"
        open={visible}
        onOk={() => form.submit()}
        onCancel={onCancel}
        confirmLoading={confirmLoading}
        okText="拉取"
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item
            name="gitUrl"
            label="GIT 地址"
            extra="仅支持SSH格式，例如 git@github.com:owner/repo.git"
            rules={[
              { required: true, message: '请输入 GIT 地址' },
              {
                validator: (_, value) => {
                  if (!value || sshGitUrlPattern.test(value.trim())) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('仅支持 SSH 格式的 Git URL'));
                },
              },
            ]}
          >
            <Input placeholder="git@github.com:owner/repo.git" autoComplete="off" onChange={handleGitUrlChange} />
          </Form.Item>
          <Form.Item label="获取分支/Tag">
            <Button onClick={handleFetchBranches} loading={branchLoading}>
              获取分支/Tag
            </Button>
          </Form.Item>
          <Form.Item name="branch" label="分支/Tag" rules={[{ required: true, message: '请选择分支或 Tag' }]}>
            <Select
              placeholder="先点击获取分支/Tag"
              options={branchOptions}
              notFoundContent="未获取分支/Tag"
            />
          </Form.Item>
          <Form.Item name="name" label="仓库名称">
            <Input placeholder="可选，自动从 GIT URL 填充" autoComplete="off" />
          </Form.Item>
          <Form.Item label="目标路径" required>
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="targetPath" noStyle rules={[{ required: true, message: '请选择目标路径' }]}>
                <Input placeholder={getTargetPathPlaceholder()} autoComplete="off" />
              </Form.Item>
              <Button onClick={() => setPathSelectorVisible(true)}>浏览</Button>
            </Space.Compact>
          </Form.Item>
          <Text type="secondary">若不设则存到工作空间/{"{仓库名称}"}</Text>
        </Form>
      </Modal>
      <PathSelector
        visible={pathSelectorVisible}
        title="选择克隆目标路径"
        placeholder="输入或选择克隆目标路径..."
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

export default CloneModal;
