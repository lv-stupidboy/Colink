import React, { useEffect, useState } from 'react';
import { Modal, Form, Input, message, Button, Select, Space, Typography } from 'antd';
import api from '@/api/client';
import type { LocalRepo, RemoteBranch } from '@/types/localRepo';

const { Text } = Typography;

interface BranchOptionGroup {
  label: string;
  options: { label: string; value: string }[];
}

interface GitConfigModalProps {
  visible: boolean;
  repo: LocalRepo | null;
  onSuccess: () => void;
  onCancel: () => void;
}

const sshGitUrlPattern = /^(ssh:\/\/[^\s@]+@[^\s/]+\/.+|[^\s/@]+@[^\s:]+:.+)$/;

const GitConfigModal: React.FC<GitConfigModalProps> = ({ visible, repo, onSuccess, onCancel }) => {
  const [form] = Form.useForm();
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [branchLoading, setBranchLoading] = useState(false);
  const [branchOptions, setBranchOptions] = useState<BranchOptionGroup[]>([]);

  useEffect(() => {
    if (visible && repo) {
      form.setFieldsValue({ gitUrl: repo.gitUrl || '', branch: repo.branch || undefined });
      setBranchOptions(repo.branch ? [{ label: '当前', options: [{ label: repo.branch, value: repo.branch }] }] : []);
    }
  }, [visible, repo]);

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
        groups.push({ label: '分支', options: branchItems.map(b => ({ label: b.name, value: b.name })) });
      }
      if (tagItems.length > 0) {
        groups.push({ label: 'Tag', options: tagItems.map(b => ({ label: b.name, value: b.name })) });
      }
      setBranchOptions(groups);
      form.setFieldsValue({ branch: branchItems[0]?.name || tagItems[0]?.name });
    } catch (error: any) {
      message.error(error?.response?.data?.error || error?.message || '获取分支/Tag失败');
    } finally {
      setBranchLoading(false);
    }
  };

  const handleGitUrlChange = () => {
    setBranchOptions([]);
    form.setFieldsValue({ branch: undefined });
  };

  const handleSubmit = async (values: any) => {
    if (!repo) return;
    setConfirmLoading(true);
    try {
      await api.repos.gitConfig(repo.id, values.gitUrl, values.branch);
      message.success('GIT 配置保存成功');
      onSuccess();
    } catch (error: any) {
      message.error(error?.response?.data?.error || '保存失败');
    } finally {
      setConfirmLoading(false);
    }
  };

  return (
    <Modal
      title="配置GIT"
      open={visible}
      onOk={() => form.submit()}
      onCancel={onCancel}
      confirmLoading={confirmLoading}
      okText="保存"
      width={560}
    >
      <Form form={form} layout="vertical" onFinish={handleSubmit}>
        <Form.Item
          label="GIT地址"
          required
        >
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item
              name="gitUrl"
              noStyle
              rules={[
                { required: true, message: '请输入 GIT地址' },
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
            <Button onClick={handleFetchBranches} loading={branchLoading}>获取分支/Tag</Button>
          </Space.Compact>
        </Form.Item>
        <Text type="secondary">仅支持SSH格式</Text>
        <Form.Item name="branch" label="分支/Tag" rules={[{ required: true, message: '请选择分支或 Tag' }]} style={{ marginTop: 16 }}>
          <Select
            placeholder="先点击获取分支/Tag"
            options={branchOptions}
            notFoundContent="未获取分支/Tag"
          />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default GitConfigModal;
