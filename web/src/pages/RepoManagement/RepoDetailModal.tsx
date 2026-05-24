import React from 'react';
import { Modal, Descriptions, Tag, Alert } from 'antd';
import type { LocalRepo, RepoStatus } from '@/types/localRepo';

interface RepoDetailModalProps {
  visible: boolean;
  repo: LocalRepo | null;
  onClose: () => void;
}

const getStatusTag = (status: RepoStatus) => {
  const config: Record<RepoStatus, { color: string; text: string }> = {
    pending: { color: 'default', text: '待就绪' },
    ready: { color: 'green', text: '就绪' },
    syncing: { color: 'processing', text: '同步中' },
    error: { color: 'red', text: '错误' },
  };
  const c = config[status] || { color: 'default', text: status };
  return <Tag color={c.color}>{c.text}</Tag>;
};

const RepoDetailModal: React.FC<RepoDetailModalProps> = ({ visible, repo, onClose }) => {
  if (!repo) return null;

  return (
    <Modal
      title="仓库详情"
      open={visible}
      onCancel={onClose}
      width={780}
      footer={<button className="ant-btn" onClick={onClose}>关闭</button>}
    >
      {repo.errorMessage && (
        <Alert type="error" message={repo.errorMessage} style={{ marginBottom: 16 }} />
      )}
      <Descriptions layout="vertical" column={2}>
        <Descriptions.Item label="仓库名称">
          <span style={{ fontWeight: 600 }}>{repo.name}</span>
        </Descriptions.Item>
        <Descriptions.Item label="状态">
          {getStatusTag(repo.status)}
        </Descriptions.Item>
        <Descriptions.Item label="GIT地址" span={2}>
          {repo.gitUrl ? (
            <a href={repo.gitUrl} target="_blank" rel="noopener noreferrer">{repo.gitUrl}</a>
          ) : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="代码仓路径">{repo.localPath}</Descriptions.Item>
        <Descriptions.Item label="当前分支">{repo.branch || '—'}</Descriptions.Item>
        <Descriptions.Item label="最新提交">{repo.lastCommit || '—'}</Descriptions.Item>
        <Descriptions.Item label="创建时间">
          {new Date(repo.createdAt).toLocaleString()}
        </Descriptions.Item>
        <Descriptions.Item label="更新时间">
          {new Date(repo.updatedAt).toLocaleString()}
        </Descriptions.Item>
      </Descriptions>
    </Modal>
  );
};

export default RepoDetailModal;
