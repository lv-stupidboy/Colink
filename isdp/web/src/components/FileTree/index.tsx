import React, { useState, useEffect, useCallback } from 'react';
import {
  Tree,
  Spin,
  Empty,
  message,
  Button,
  Tooltip,
} from 'antd';
import {
  FolderOutlined,
  FileOutlined,
  FolderOpenOutlined,
  ReloadOutlined,
  FileTextOutlined,
  CodeOutlined,
  FileMarkdownOutlined,
  FileImageOutlined,
  FilePdfOutlined,
  FileZipOutlined,
} from '@ant-design/icons';
import type { DataNode, TreeProps } from 'antd/es/tree';
import api from '@/api/client';
import type { FileInfo, ListFilesResponse } from '@/types';
import './FileTree.css';

interface FileTreeProps {
  projectId: string;
  projectPath?: string; // 调试模式下使用
  onFileSelect?: (path: string, isDir: boolean) => void;
  style?: React.CSSProperties;
}

/**
 * FileTree component - displays project file structure
 * 支持两种模式：
 * - 团队模式：projectId 为实际项目ID，使用项目API加载文件
 * - 调试模式：projectId 为 'debug'，需要传入 projectPath，使用路径API加载文件
 */
const FileTree: React.FC<FileTreeProps> = ({ projectId, projectPath, onFileSelect, style }) => {
  const [loading, setLoading] = useState(false);
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  const [loadedKeys, setLoadedKeys] = useState<Set<string>>(new Set());

  // 是否为调试模式
  const isDebugMode = projectId === 'debug';

  // Get file icon based on extension
  const getFileIcon = (name: string, isDir: boolean): React.ReactNode => {
    if (isDir) return <FolderOutlined />;
    const ext = name.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'js':
      case 'jsx':
      case 'ts':
      case 'tsx':
        return <CodeOutlined style={{ color: '#1890ff' }} />;
      case 'json':
        return <CodeOutlined style={{ color: '#faad14' }} />;
      case 'md':
      case 'markdown':
        return <FileMarkdownOutlined style={{ color: '#52c41a' }} />;
      case 'txt':
      case 'log':
        return <FileTextOutlined style={{ color: '#666' }} />;
      case 'png':
      case 'jpg':
      case 'jpeg':
      case 'gif':
      case 'svg':
        return <FileImageOutlined style={{ color: '#eb2f96' }} />;
      case 'pdf':
        return <FilePdfOutlined style={{ color: '#f5222d' }} />;
      case 'zip':
      case 'tar':
      case 'gz':
        return <FileZipOutlined style={{ color: '#722ed1' }} />;
      case 'go':
        return <CodeOutlined style={{ color: '#00add8' }} />;
      case 'py':
        return <CodeOutlined style={{ color: '#3776ab' }} />;
      case 'java':
        return <CodeOutlined style={{ color: '#f89820' }} />;
      default:
        return <FileOutlined style={{ color: '#666' }} />;
    }
  };

  // Convert FileInfo to DataNode
  const convertToDataNode = (file: FileInfo): DataNode => {
    const isLeaf = !file.isDir;
    return {
      key: file.path,
      title: file.name,
      icon: getFileIcon(file.name, file.isDir),
      isLeaf,
      children: isLeaf ? undefined : [],
    };
  };

  // Load files from API
  const loadFiles = useCallback(async (path: string = ''): Promise<FileInfo[]> => {
    try {
      let response: ListFilesResponse;
      if (isDebugMode) {
        // 调试模式：使用路径API
        if (!projectPath) {
          message.warning('请先设置工作目录');
          return [];
        }
        response = await api.projects.browseFiles(projectPath, path);
      } else {
        // 团队模式：使用项目API
        response = await api.projects.listFiles(projectId, path);
      }
      return response.files || [];
    } catch (error) {
      console.error('Failed to load files:', error);
      message.error('加载文件列表失败');
      return [];
    }
  }, [projectId, projectPath, isDebugMode]);

  // Load root directory
  const loadRootDirectory = useCallback(async () => {
    // 调试模式下，没有 projectPath 时直接返回
    if (isDebugMode && !projectPath) {
      setTreeData([]);
      return;
    }
    setLoading(true);
    try {
      const files = await loadFiles('');
      const nodes = files.map(convertToDataNode);
      setTreeData(nodes);
      // Auto-expand all directories by default
      const dirKeys = files.filter(f => f.isDir).map(f => f.path);
      setExpandedKeys(dirKeys);
    } finally {
      setLoading(false);
    }
  }, [loadFiles, isDebugMode, projectPath]);

  // Initial load - 依赖 projectPath 变化时重新加载
  useEffect(() => {
    loadRootDirectory();
    // 重置已加载的keys
    setLoadedKeys(new Set());
  }, [loadRootDirectory]);

  // Handle tree node expansion - load children on demand
  const onLoadData: TreeProps['loadData'] = async ({ key, children }) => {
    // Skip if already loaded
    if (children && children.length > 0) return;
    if (loadedKeys.has(key as string)) return;

    const path = key as string;
    const files = await loadFiles(path);

    // Find and update the node in treeData
    const updateNodeChildren = (nodes: DataNode[]): DataNode[] => {
      return nodes.map(node => {
        if (node.key === key) {
          return {
            ...node,
            children: files.map(convertToDataNode),
          };
        }
        if (node.children) {
          return {
            ...node,
            children: updateNodeChildren(node.children),
          };
        }
        return node;
      });
    };

    setTreeData(prev => updateNodeChildren(prev));
    setLoadedKeys(prev => new Set(prev).add(key as string));
  };

  // Handle node selection
  const onSelect: TreeProps['onSelect'] = (selectedKeys, info) => {
    if (selectedKeys.length > 0) {
      const path = selectedKeys[0] as string;
      const node = info.node;
      const isDir = !node.isLeaf;
      onFileSelect?.(path, isDir);
    }
  };

  // Handle expand/collapse
  const onExpand: TreeProps['onExpand'] = (expandedKeys) => {
    setExpandedKeys(expandedKeys);
  };

  // Refresh tree
  const handleRefresh = () => {
    setLoadedKeys(new Set());
    setExpandedKeys([]);
    loadRootDirectory();
  };

  if (loading) {
    return (
      <div className="file-tree-loading">
        <Spin size="small" />
        <span>加载中...</span>
      </div>
    );
  }

  return (
    <div className="file-tree" style={style}>
      <div className="file-tree-header">
        <span className="file-tree-title">项目文件</span>
        <Tooltip title="刷新">
          <Button
            type="text"
            size="small"
            icon={<ReloadOutlined />}
            onClick={handleRefresh}
          />
        </Tooltip>
      </div>
      {treeData.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={isDebugMode && !projectPath ? '请先设置工作目录' : '暂无文件'}
          style={{ padding: '20px 0' }}
        />
      ) : (
        <Tree
          showIcon
          blockNode
          expandedKeys={expandedKeys}
          onExpand={onExpand}
          loadData={onLoadData}
          onSelect={onSelect}
          treeData={treeData}
          className="file-tree-content"
          switcherIcon={<FolderOpenOutlined />}
        />
      )}
    </div>
  );
};

export default FileTree;