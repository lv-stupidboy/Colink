import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Button, Empty, Modal, Spin, Tag, Tooltip, message } from 'antd';
import {
  ArrowsAltOutlined,
  CheckOutlined,
  CopyOutlined,
  DatabaseOutlined,
  FileMarkdownOutlined,
  ReloadOutlined,
  RightOutlined,
} from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import api from '@/api/client';
import type { RawMemoryGroup, RawMemoryResponse } from '@/types';

interface Props {
  scope?: {
    teamId?: string;
    teamName?: string;
    projectId?: string;
    projectName?: string;
    workspacePath?: string;
  };
}

type MemoryGroupKey = 'team' | 'project';

interface MemoryDocument {
  key: string;
  name: string;
  path?: string;
  content: string;
  isIndex?: boolean;
}

interface MarkdownViewProps {
  content: string;
  className: string;
  emptyText: string;
  documents: MemoryDocument[];
  onOpenDocument: (key: string) => void;
}

const groupLabels: Record<MemoryGroupKey, string> = {
  team: '团队记忆',
  project: '项目记忆',
};

const normalizeLinkPath = (value: string): string => {
  const cleanValue = value.split('#')[0].split('?')[0].trim();
  try {
    return decodeURIComponent(cleanValue).replace(/\\/g, '/').toLowerCase();
  } catch {
    return cleanValue.replace(/\\/g, '/').toLowerCase();
  }
};

const normalizeMemoryMarkdown = (content: string): string => {
  const normalizedLineBreaks = content.replace(/\r\n/g, '\n');
  const withFrontMatterBlock = normalizedLineBreaks.replace(
    /^---\n([\s\S]*?)\n---(?=\n|$)/,
    (_match, frontMatter: string) => `\`\`\`yaml\n---\n${frontMatter}\n---\n\`\`\``
  );
  return withFrontMatterBlock
    .replace(/([^\n])\n(#{1,6}\s)/g, '$1\n\n$2')
    .replace(/^(-\s+)([A-Za-z][A-Za-z0-9 _-]{1,40}):\s+/gm, '$1**$2**: ');
};

const buildDocuments = (groupKey: MemoryGroupKey, group?: RawMemoryGroup): MemoryDocument[] => {
  if (!group) return [];
  const docs: MemoryDocument[] = [];
  if (group.indexPath || group.index) {
    docs.push({
      key: `${groupKey}:index`,
      name: 'MEMORY.md',
      path: group.indexPath,
      content: group.index || '',
      isIndex: true,
    });
  }
  for (const file of group.files || []) {
    docs.push({
      key: `${groupKey}:${file.path || file.name}`,
      name: file.name,
      path: file.path,
      content: file.content,
    });
  }
  return docs;
};

const findDocumentByHref = (href: string, documents: MemoryDocument[]): MemoryDocument | undefined => {
  const normalizedHref = normalizeLinkPath(href);
  if (!normalizedHref.endsWith('.md')) return undefined;
  return documents.find((doc) => {
    const normalizedName = normalizeLinkPath(doc.name);
    const normalizedPath = doc.path ? normalizeLinkPath(doc.path) : '';
    return normalizedHref === normalizedName || normalizedPath.endsWith(`/${normalizedHref}`) || normalizedPath.endsWith(normalizedHref);
  });
};

const MarkdownView: React.FC<MarkdownViewProps> = ({ content, className, emptyText, documents, onOpenDocument }) => {
  const normalizedContent = normalizeMemoryMarkdown(content.trim());
  if (!normalizedContent) {
    return <Empty className={className} description={emptyText} image={Empty.PRESENTED_IMAGE_SIMPLE} />;
  }
  return (
    <div className={className}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ href, children }) => {
            const targetDoc = href ? findDocumentByHref(href, documents) : undefined;
            if (targetDoc) {
              return (
                <button className="memory-markdown-link" type="button" onClick={() => onOpenDocument(targetDoc.key)}>
                  {children}
                </button>
              );
            }
            return (
              <a href={href} target="_blank" rel="noreferrer">
                {children}
              </a>
            );
          },
        }}
      >
        {normalizedContent}
      </ReactMarkdown>
    </div>
  );
};

export const MemoryEntriesPanel: React.FC<Props> = ({ scope }) => {
  const [data, setData] = useState<RawMemoryResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [collapsedGroups, setCollapsedGroups] = useState<Record<MemoryGroupKey, boolean>>({
    team: false,
    project: false,
  });
  const [reloadKey, setReloadKey] = useState(0);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [modalGroup, setModalGroup] = useState<MemoryGroupKey | null>(null);
  const [activeDocKey, setActiveDocKey] = useState('');

  useEffect(() => {
    const hasScope = Boolean(scope?.teamId || scope?.workspacePath);
    if (!hasScope) {
      setData(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);
    api.memory.raw('all', scope)
      .then((response) => {
        if (!cancelled) {
          setData(response);
        }
      })
      .catch((err: { message?: string }) => {
        if (!cancelled) {
          setError(err.message || '加载记忆失败');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [scope?.teamId, scope?.teamName, scope?.projectId, scope?.projectName, scope?.workspacePath, reloadKey]);

  const totalFiles = useMemo(() => {
    if (!data) return 0;
    return (data.team?.files?.length || 0) + (data.project?.files?.length || 0);
  }, [data]);

  const activeDocs = useMemo(() => {
    if (!modalGroup) return [];
    return buildDocuments(modalGroup, data?.[modalGroup]);
  }, [data, modalGroup]);

  const activeDoc = useMemo(() => {
    if (activeDocs.length === 0) return null;
    return activeDocs.find(doc => doc.key === activeDocKey) || activeDocs[0];
  }, [activeDocs, activeDocKey]);

  const toggleGroup = useCallback((group: MemoryGroupKey) => {
    setCollapsedGroups(prev => ({ ...prev, [group]: !prev[group] }));
  }, []);

  const copyContent = useCallback(async (key: string, content: string, event?: React.MouseEvent) => {
    event?.stopPropagation();
    await navigator.clipboard.writeText(content);
    setCopiedKey(key);
    message.success('已复制记忆内容');
    setTimeout(() => setCopiedKey(null), 2000);
  }, []);

  const openDocumentInModal = useCallback((groupKey: MemoryGroupKey, docKey: string) => {
    setModalGroup(groupKey);
    setActiveDocKey(docKey);
  }, []);

  const openModal = useCallback((groupKey: MemoryGroupKey, group?: RawMemoryGroup, event?: React.MouseEvent) => {
    event?.stopPropagation();
    const docs = buildDocuments(groupKey, group);
    setModalGroup(groupKey);
    setActiveDocKey(docs[0]?.key || '');
  }, []);

  const renderGroup = (groupKey: MemoryGroupKey, group?: RawMemoryGroup) => {
    const files = group?.files || [];
    const missing = group?.missing || [];
    const collapsed = collapsedGroups[groupKey];
    const indexContent = group?.index || '';
    const documents = buildDocuments(groupKey, group);
    const scopeName = groupKey === 'team'
      ? data?.scope?.teamName || data?.scope?.teamId || '未绑定团队'
      : data?.scope?.projectName || data?.scope?.workspacePath || '未绑定项目';

    return (
      <div className="memory-group" key={groupKey}>
        <div className="memory-group-header" onClick={() => toggleGroup(groupKey)}>
          <div className="memory-group-title">
            <RightOutlined className={`memory-expand-icon ${collapsed ? '' : 'expanded'}`} />
            <span>{groupLabels[groupKey]}</span>
            <Tag className="memory-count-tag">{files.length}</Tag>
          </div>
          <Tooltip title={scopeName}>
            <span className="memory-scope">{scopeName}</span>
          </Tooltip>
        </div>

        {!collapsed && (
          <div className="memory-group-body">
            {group?.indexPath && (
              <Tooltip title={group.indexPath}>
                <div className="memory-index-path">{group.indexExists ? group.indexPath : `${group.indexPath}（不存在）`}</div>
              </Tooltip>
            )}

            <div className="memory-index-toolbar">
              <Button size="small" icon={<ArrowsAltOutlined />} onClick={(event) => openModal(groupKey, group, event)}>
                放大
              </Button>
              <Button
                size="small"
                icon={copiedKey === `${groupKey}:index` ? <CheckOutlined /> : <CopyOutlined />}
                onClick={(event) => copyContent(`${groupKey}:index`, indexContent, event)}
              />
            </div>

            {group?.indexExists ? (
              <MarkdownView
                className="memory-index-content"
                content={indexContent}
                emptyText="MEMORY.md 为空"
                documents={documents}
                onOpenDocument={(docKey) => openDocumentInModal(groupKey, docKey)}
              />
            ) : (
              <Empty
                description={group?.indexPath ? '记忆索引文件不存在' : '未解析到记忆路径'}
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            )}

            {missing.length > 0 && (
              <Alert
                type="warning"
                showIcon
                message={`索引中有 ${missing.length} 个文件不存在`}
                description={missing.join(', ')}
              />
            )}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="status-section memory-section">
      <div className="status-section-title">
        <DatabaseOutlined />
        <span>记忆</span>
        {totalFiles > 0 && <Tag className="memory-total-tag">{totalFiles}</Tag>}
        {(scope?.workspacePath || scope?.teamId) && (
          <Tooltip title="刷新记忆">
            <span className="memory-refresh" onClick={() => setReloadKey(value => value + 1)}>
              <ReloadOutlined />
            </span>
          </Tooltip>
        )}
      </div>

      {!scope?.workspacePath && !scope?.teamId ? (
        <Empty description="暂无记忆上下文" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : loading ? (
        <div className="memory-loading">
          <Spin size="small" />
          <span>加载记忆中...</span>
        </div>
      ) : error ? (
        <Alert type="error" showIcon message={error} />
      ) : (
        <>
          {renderGroup('team', data?.team)}
          {renderGroup('project', data?.project)}
        </>
      )}

      <Modal
        title={modalGroup ? groupLabels[modalGroup] : '记忆'}
        open={Boolean(modalGroup)}
        onCancel={() => setModalGroup(null)}
        footer={null}
        width={980}
        className="memory-modal"
        destroyOnClose
      >
        <div className="memory-modal-layout">
          <div className="memory-modal-sidebar">
            {activeDocs.map(doc => (
              <div
                key={doc.key}
                className={`memory-modal-file ${activeDoc?.key === doc.key ? 'active' : ''}`}
                onClick={() => setActiveDocKey(doc.key)}
              >
                <FileMarkdownOutlined />
                <span>{doc.name}</span>
                {doc.isIndex && <Tag>索引</Tag>}
              </div>
            ))}
          </div>
          <div className="memory-modal-content">
            {activeDoc ? (
              <>
                <div className="memory-modal-content-header">
                  <div className="memory-modal-file-title">
                    <FileMarkdownOutlined />
                    <span>{activeDoc.name}</span>
                  </div>
                  <div className="memory-modal-actions">
                    <Button
                      size="small"
                      icon={copiedKey === activeDoc.key ? <CheckOutlined /> : <CopyOutlined />}
                      onClick={(event) => copyContent(activeDoc.key, activeDoc.content, event)}
                    >
                      复制
                    </Button>
                  </div>
                </div>
                {activeDoc.path && <div className="memory-modal-path">{activeDoc.path}</div>}
                <MarkdownView
                  className="memory-modal-markdown"
                  content={activeDoc.content}
                  emptyText="文件为空"
                  documents={activeDocs}
                  onOpenDocument={setActiveDocKey}
                />
              </>
            ) : (
              <Empty description="暂无记忆文件" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default MemoryEntriesPanel;
