import React, { useEffect } from 'react';
import { Table, Card, Button, Space, Input, Popconfirm, message } from 'antd';
import { UploadOutlined, CloudDownloadOutlined, SearchOutlined } from '@ant-design/icons';
import { useRepoStore } from '@/store/repoStore';
import RepoDetailModal from './RepoDetailModal';
import UploadModal from './UploadModal';
import CloneModal from './CloneModal';
import GitConfigModal from './GitConfigModal';
import type { LocalRepo } from '@/types/localRepo';
import './RepoManagement.css';

const RepoManagement: React.FC = () => {
  const { loading, searchKeyword, fetchRepos, deleteRepo, syncRepo, setSearchKeyword, getFilteredRepos } = useRepoStore();

  const [detailVisible, setDetailVisible] = React.useState(false);
  const [detailRepo, setDetailRepo] = React.useState<LocalRepo | null>(null);
  const [uploadVisible, setUploadVisible] = React.useState(false);
  const [cloneVisible, setCloneVisible] = React.useState(false);
  const [gitConfigVisible, setGitConfigVisible] = React.useState(false);
  const [gitConfigRepo, setGitConfigRepo] = React.useState<LocalRepo | null>(null);

  useEffect(() => {
    fetchRepos().catch(() => {
      message.error('加载仓库列表失败');
    });
  }, []);

  const handleSync = async (repo: LocalRepo) => {
    if (!repo.gitUrl) {
      message.warning('该仓库未配置 GIT 地址，请先配置 GIT');
      setGitConfigRepo(repo);
      setGitConfigVisible(true);
      return;
    }
    try {
      await syncRepo(repo.id);
      message.success('同步成功');
    } catch {
      message.error('同步失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteRepo(id);
      message.success('仓库已删除');
    } catch {
      message.error('删除失败');
    }
  };

  const handleRefresh = () => {
    fetchRepos().catch(() => {
      message.error('刷新列表失败');
    });
  };

  const columns = [
    {
      title: '仓库名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (name: string, record: LocalRepo) => (
        <span
          className="repo-name"
          onClick={() => {
            setDetailRepo(record);
            setDetailVisible(true);
          }}
        >
          {name}
        </span>
      ),
    },
    {
      title: '代码路径',
      dataIndex: 'localPath',
      key: 'localPath',
      ellipsis: true,
      render: (path: string) => <span className="repo-path">{path}</span>,
    },
    {
      title: 'GIT 地址',
      dataIndex: 'gitUrl',
      key: 'gitUrl',
      width: 280,
      render: (gitUrl?: string) =>
        gitUrl ? (
          <span className="repo-git-url" title={gitUrl}>{gitUrl}</span>
        ) : (
          <span className="repo-no-git">—</span>
        ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 220,
      fixed: 'right' as const,
      render: (_: unknown, record: LocalRepo) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              setDetailRepo(record);
              setDetailVisible(true);
            }}
          >
            详情
          </Button>
          <Button
            type="link"
            onClick={() => handleSync(record)}
          >
            同步
          </Button>
          <Button
            type="link"
            onClick={() => {
              setGitConfigRepo(record);
              setGitConfigVisible(true);
            }}
          >
            配置GIT
          </Button>
          <Popconfirm
            title="确定删除此仓库？"
            description="删除后无法恢复"
            onConfirm={() => handleDelete(record.id)}
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button type="link" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="repo-management">
      <div className="page-header">
        <div className="title-section">
          <h2>代码仓管理</h2>
          <div className="desc">管理本地代码仓库，上传 ZIP 包或拉取远程代码仓</div>
        </div>
        <div className="actions">
          <Button icon={<UploadOutlined />} onClick={() => setUploadVisible(true)}>
            导入本地文件
          </Button>
          <Button type="primary" icon={<CloudDownloadOutlined />} onClick={() => setCloneVisible(true)}>
            远程代码仓拉取
          </Button>
        </div>
      </div>

      <div className="search-bar">
        <Input.Search
          placeholder="搜索仓库名称"
          allowClear
          prefix={<SearchOutlined />}
          value={searchKeyword}
          onChange={(e) => setSearchKeyword(e.target.value)}
          onSearch={(value) => setSearchKeyword(value)}
          style={{ width: 300 }}
        />
      </div>

      <Card>
        <Table
          dataSource={getFilteredRepos()}
          columns={columns}
          rowKey="id"
          loading={loading}
          scroll={{ x: 'max-content' }}
        />
      </Card>

      <RepoDetailModal
        visible={detailVisible}
        repo={detailRepo}
        onClose={() => setDetailVisible(false)}
      />

      <UploadModal
        visible={uploadVisible}
        onSuccess={() => {
          setUploadVisible(false);
          handleRefresh();
        }}
        onCancel={() => setUploadVisible(false)}
      />

      <CloneModal
        visible={cloneVisible}
        onSuccess={() => {
          setCloneVisible(false);
          handleRefresh();
        }}
        onCancel={() => setCloneVisible(false)}
      />

      <GitConfigModal
        visible={gitConfigVisible}
        repo={gitConfigRepo}
        onSuccess={() => {
          setGitConfigVisible(false);
          handleRefresh();
        }}
        onCancel={() => setGitConfigVisible(false)}
      />
    </div>
  );
};

export default RepoManagement;
