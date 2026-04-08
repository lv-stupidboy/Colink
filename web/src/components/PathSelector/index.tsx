import React, { useState, useEffect } from 'react';
import {
  Modal,
  Input,
  List,
  Button,
  Space,
  Breadcrumb,
  Alert,
  Spin,
  Typography,
  Empty,
} from 'antd';
import {
  FolderOutlined,
  FileOutlined,
  DesktopOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import api from '@/api/client';

const { Text } = Typography;

interface PathSelectorProps {
  visible: boolean;
  onSelect: (path: string) => void;
  onCancel: () => void;
  title?: string;
}

interface FileEntry {
  name: string;
  path: string;
  isDir: boolean;
}

interface BrowseResult {
  currentPath: string;
  parentPath: string;
  entries: FileEntry[];
  drives?: string[];
  isValid: boolean;
  error?: string;
}

const PathSelector: React.FC<PathSelectorProps> = ({
  visible,
  onSelect,
  onCancel,
  title = '选择项目路径',
}) => {
  const [currentPath, setCurrentPath] = useState('');
  const [browseResult, setBrowseResult] = useState<BrowseResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [manualPath, setManualPath] = useState('');
  const [newFolderModalVisible, setNewFolderModalVisible] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [creatingFolder, setCreatingFolder] = useState(false);

  // 初始化加载驱动器列表（Windows）
  useEffect(() => {
    if (visible) {
      browsePath('');
    }
  }, [visible]);

  // 浏览路径
  const browsePath = async (path: string) => {
    setLoading(true);
    try {
      const result = await api.files.browse(path);
      setBrowseResult(result);
      setCurrentPath(result.currentPath);
      setManualPath(result.currentPath);
    } catch (error) {
      console.error('Failed to browse path:', error);
    } finally {
      setLoading(false);
    }
  };

  // 进入目录
  const handleEnterDir = (entry: FileEntry) => {
    if (entry.isDir) {
      browsePath(entry.path);
    }
  };

  // 确认选择
  const handleConfirm = () => {
    const pathToUse = manualPath || currentPath;
    if (pathToUse) {
      onSelect(pathToUse);
      // 重置状态
      setCurrentPath('');
      setBrowseResult(null);
      setManualPath('');
    }
  };

  // 取消
  const handleCancel = () => {
    setCurrentPath('');
    setBrowseResult(null);
    setManualPath('');
    onCancel();
  };

  // 创建新文件夹
  const handleCreateFolder = async () => {
    if (!newFolderName.trim()) return;

    setCreatingFolder(true);
    try {
      await api.files.createFolder(currentPath, newFolderName.trim());
      setNewFolderModalVisible(false);
      setNewFolderName('');
      // 刷新当前目录
      browsePath(currentPath);
    } catch (error: any) {
      console.error('Failed to create folder:', error);
    } finally {
      setCreatingFolder(false);
    }
  };

  return (
    <Modal
      title={title}
      open={visible}
      onOk={handleConfirm}
      onCancel={handleCancel}
      width={700}
      okText="选择此路径"
      cancelText="取消"
    >
      {/* 手动输入路径 */}
      <Input
        placeholder="输入或选择项目路径..."
        value={manualPath}
        onChange={(e) => setManualPath(e.target.value)}
        style={{ marginBottom: 16 }}
      />

      {/* 浏览区域 */}
      <div style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          <Text strong>浏览文件夹</Text>
          {currentPath && (
            <Button
              type="link"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => setNewFolderModalVisible(true)}
            >
              新建文件夹
            </Button>
          )}
        </div>

        {/* 面包屑导航 */}
        {currentPath && (
          <div style={{ marginBottom: 8, display: 'flex', alignItems: 'center' }}>
            <Button type="link" size="small" onClick={() => browsePath('')}>
              <DesktopOutlined /> 我的电脑
            </Button>
            {currentPath.split(/[\\/]/).filter(Boolean).length > 0 && (
              <>
                <Text type="secondary"> / </Text>
                <Breadcrumb separator="/">
                  {currentPath.split(/[\\/]/).filter(Boolean).map((part, index, arr) => (
                    <Breadcrumb.Item key={index}>
                      {index === arr.length - 1 ? (
                        <Text strong>{part}</Text>
                      ) : (
                        <a
                          onClick={() => {
                            const parts = currentPath.split(/[\\/]/).filter(Boolean);
                            const newPath = parts.slice(0, index + 1).join('/');
                            browsePath(newPath.match(/^[A-Za-z]:/) ? newPath : '/' + newPath);
                          }}
                        >
                          {part}
                        </a>
                      )}
                    </Breadcrumb.Item>
                  ))}
                </Breadcrumb>
              </>
            )}
          </div>
        )}

        {/* 文件列表 */}
        <div
          style={{
            border: '1px solid #d9d9d9',
            borderRadius: 4,
            maxHeight: 300,
            overflowY: 'auto',
          }}
        >
          {loading ? (
            <div style={{ textAlign: 'center', padding: 40 }}>
              <Spin />
            </div>
          ) : browseResult?.error ? (
            <Alert type="error" message={browseResult.error} style={{ margin: 16 }} />
          ) : browseResult?.drives ? (
            // Windows 驱动器列表
            <List
              dataSource={browseResult.drives}
              renderItem={(drive) => (
                <List.Item
                  style={{ padding: '12px 16px', cursor: 'pointer' }}
                  onClick={() => browsePath(drive)}
                >
                  <Space>
                    <DesktopOutlined style={{ fontSize: 18, color: '#1890ff' }} />
                    <Text>{drive}</Text>
                  </Space>
                </List.Item>
              )}
            />
          ) : browseResult?.entries?.length ? (
            <List
              dataSource={browseResult.entries}
              renderItem={(entry) => (
                <List.Item
                  style={{ padding: '12px 16px', cursor: 'pointer' }}
                  onClick={() => handleEnterDir(entry)}
                >
                  <Space>
                    {entry.isDir ? (
                      <FolderOutlined style={{ fontSize: 18, color: '#faad14' }} />
                    ) : (
                      <FileOutlined style={{ fontSize: 18 }} />
                    )}
                    <Text>{entry.name}</Text>
                  </Space>
                </List.Item>
              )}
            />
          ) : (
            <Empty description="此目录为空" style={{ padding: 40 }} />
          )}
        </div>
      </div>

      {/* 新建文件夹弹窗 */}
      <Modal
        title="新建文件夹"
        open={newFolderModalVisible}
        onOk={handleCreateFolder}
        onCancel={() => {
          setNewFolderModalVisible(false);
          setNewFolderName('');
        }}
        confirmLoading={creatingFolder}
        okText="创建"
        cancelText="取消"
      >
        <Input
          placeholder="请输入文件夹名称"
          value={newFolderName}
          onChange={(e) => setNewFolderName(e.target.value)}
          onPressEnter={handleCreateFolder}
        />
      </Modal>
    </Modal>
  );
};

export default PathSelector;