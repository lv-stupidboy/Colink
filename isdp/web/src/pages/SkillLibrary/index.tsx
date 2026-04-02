import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Card, Button, Modal, Form, Input, Select, message, Space, Tag, Typography,
  Popconfirm, Empty, Spin, Divider, Tooltip, Radio, Pagination
} from 'antd';
import {
  PlusOutlined,
  EyeOutlined,
  EditOutlined,
  DeleteOutlined,
  LinkOutlined,
  CloudUploadOutlined,
  CloudDownloadOutlined,
  FolderOpenOutlined
} from '@ant-design/icons';
import JSZip from 'jszip';
import api from '@/api/client';
import type { Skill, SkillSourceType, BuiltInTagCategory, SkillRegistry } from '@/types';

const { Title, Text, Paragraph } = Typography;
const { CheckableTag: CheckableTagAnt } = Tag;

// 内置标签分类
const builtInTagCategories: BuiltInTagCategory[] = [
  { name: '编程语言', tags: ['Java', 'Python', 'JavaScript', 'TypeScript', 'Go', 'Rust', 'C++', 'C#'] },
  { name: '前端技术', tags: ['React', 'Vue', 'Angular', 'Next.js', 'CSS', 'Tailwind', 'Webpack', 'Vite'] },
  { name: '后端技术', tags: ['Spring', 'Django', 'Flask', 'Express', 'Gin', 'FastAPI', 'Node.js'] },
  { name: '数据库', tags: ['MySQL', 'PostgreSQL', 'MongoDB', 'Redis', 'Elasticsearch', 'SQLite'] },
  { name: '云与DevOps', tags: ['Docker', 'Kubernetes', 'AWS', 'Azure', 'GCP', 'CI/CD', 'Terraform'] },
  { name: '使用场景', tags: ['代码规范', '代码审查', '单元测试', '安全审计', '性能优化', '重构'] },
];

// 根据技能名称生成头像
const generateAvatar = (name: string): { initials: string; color: string } => {
  const words = name.split(/[-_\s]+/);
  let initials = '';
  if (words.length >= 2) {
    initials = (words[0][0] + words[1][0]).toUpperCase();
  } else if (name.length >= 2) {
    initials = name.substring(0, 2).toUpperCase();
  } else {
    initials = name.toUpperCase();
  }

  const colors = [
    '#1890ff', '#52c41a', '#fa8c16', '#eb2f96', '#722ed1',
    '#13c2c2', '#2f54eb', '#faad14', '#a0d911', '#f5222d'
  ];
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash);
  }
  const color = colors[Math.abs(hash) % colors.length];

  return { initials, color };
};

// 验证技能名称格式
const validateSkillName = (name: string): { valid: boolean; message?: string } => {
  if (!name) {
    return { valid: false, message: '请输入技能名称' };
  }
  const pattern = /^[a-z][a-z0-9-]*$/;
  if (!pattern.test(name)) {
    return { valid: false, message: '名称只能包含小写字母、数字和中划线，且必须以字母开头' };
  }
  if (name.includes('--')) {
    return { valid: false, message: '名称不能包含连续的中划线' };
  }
  if (name.endsWith('-')) {
    return { valid: false, message: '名称不能以中划线结尾' };
  }
  return { valid: true };
};

// 技能头像组件
const SkillAvatar: React.FC<{ name: string }> = ({ name }) => {
  const { initials, color } = generateAvatar(name);
  return (
    <div
      style={{
        width: 40,
        height: 40,
        borderRadius: 8,
        background: `linear-gradient(135deg, ${color}dd, ${color})`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: '#fff',
        fontWeight: 600,
        fontSize: 14,
        boxShadow: `0 2px 6px ${color}40`,
      }}
    >
      {initials}
    </div>
  );
};

const SkillLibrary: React.FC = () => {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [registries, setRegistries] = useState<SkillRegistry[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null);
  const [searchText, setSearchText] = useState('');
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [sourceFilter, setSourceFilter] = useState<string>('');
  const [agentTypeFilter, setAgentTypeFilter] = useState<string>('');
  const [allTags, setAllTags] = useState<string[]>([]);
  const [form] = Form.useForm();

  // 创建方式状态
  const [sourceType, setSourceType] = useState<SkillSourceType>('personal');
  const [createMethod, setCreateMethod] = useState<'upload' | 'repo'>('upload');
  const [repoUrl, setRepoUrl] = useState('');
  const [selectedRegistryId, setSelectedRegistryId] = useState<string>('');
  const [importing, setImporting] = useState(false);
  const [isAfterUpload, setIsAfterUpload] = useState(false);
  const directoryInputRef = useRef<HTMLInputElement>(null);
  const pendingZipBlobRef = useRef<Blob | null>(null); // 待上传的 zip blob
  const renderDirectoryUpload = () => (
    <>
      {/* 隐藏的目录选择 input */}
      <input
        ref={directoryInputRef}
        type="file"
        style={{ display: 'none' }}
        onChange={handleDirectoryChange}
        multiple
        // @ts-ignore webkitdirectory 属性
        webkitdirectory=""
        directory=""
      />
      <div
        onClick={handleDirectorySelect}
        style={{
          border: '1px dashed var(--ant-color-border)',
          borderRadius: 8,
          padding: '24px 0',
          textAlign: 'center',
          cursor: 'pointer',
          transition: 'border-color 0.3s',
        }}
      >
        <p>
          <FolderOpenOutlined style={{ fontSize: 32, color: 'var(--ant-color-primary)' }} />
        </p>
        <p style={{ color: 'var(--ant-color-text)' }}>点击选择技能目录</p>
        <p style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
          目录需包含 SKILL.md 文件，最大 5MB
        </p>
      </div>
    </>
  );

  // Agent 类型选项
  const agentTypeOptions = [
    { label: 'Claude Code', value: 'claude_code' },
    { label: 'OpenCode', value: 'open_code' },
  ];

  const loadSkills = useCallback(async () => {
    setLoading(true);
    try {
      const tag = selectedTags.length > 0 ? selectedTags[0] : undefined;
      const result = await api.skills.list({
        page,
        pageSize,
        search: searchText,
        tag,
        sourceType: sourceFilter,
        agentType: agentTypeFilter,
      });
      setSkills(result.data);
      setTotal(result.total);
    } catch (error) {
      message.error('加载技能列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText, selectedTags, sourceFilter, agentTypeFilter]);

  const loadTags = useCallback(async () => {
    try {
      const tags = await api.skills.getTags();
      setAllTags(tags);
    } catch (error) {
      // 忽略错误
    }
  }, []);

  const loadRegistries = useCallback(async () => {
    try {
      const result = await api.registries.list();
      setRegistries(result.data || []);
    } catch (error) {
      // 忽略错误
    }
  }, []);

  useEffect(() => {
    loadSkills();
    loadTags();
    loadRegistries();
  }, [loadSkills, loadTags, loadRegistries]);

  const handleCreate = () => {
    setEditingSkill(null);
    setSourceType('personal');
    setCreateMethod('upload');
    setRepoUrl('');
    setSelectedRegistryId('');
    setIsAfterUpload(false);
    pendingZipBlobRef.current = null;
    form.resetFields();
    form.setFieldsValue({ sourceType: 'personal', tags: [], isPublic: true, supportedAgents: [] });
    setModalVisible(true);
  };

  const handleEdit = (record: Skill) => {
    setEditingSkill(record);
    setSourceType(record.sourceType);
    setIsAfterUpload(false);
    pendingZipBlobRef.current = null;
    form.setFieldsValue({
      name: record.name,
      description: record.description,
      tags: record.tags || [],
      sourceType: record.sourceType,
      supportedAgents: record.supportedAgents || [],
      isPublic: record.isPublic,
    });
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.skills.delete(id);
      message.success('删除成功');
      loadSkills();
    } catch (error: any) {
      const errorData = error.response?.data;
      if (errorData?.error) {
        message.error(errorData.error);
      } else {
        message.error('删除失败');
      }
    }
  };

  const handleSubmit = async (values: any) => {
    const validation = validateSkillName(values.name);
    if (!validation.valid) {
      message.error(validation.message);
      return;
    }

    try {
      if (isAfterUpload && pendingZipBlobRef.current) {
        // 新上传：上传 zip 文件创建记录
        message.loading({ content: '正在创建技能...', key: 'uploading' });

        const formData = new FormData();
        formData.append('file', pendingZipBlobRef.current, 'skill.zip');
        formData.append('source_type', values.sourceType || sourceType);
        formData.append('directory_name', values.name);
        formData.append('description', values.description || ''); // 传递前端解析的描述

        const response = await fetch('/api/v1/skills/upload', {
          method: 'POST',
          body: formData,
        });

        const result = await response.json();
        message.destroy('uploading');

        if (!response.ok) {
          throw new Error(result.error || '上传失败');
        }

        // 更新额外字段（如果有）
        if (values.tags?.length || values.supportedAgents?.length) {
          await api.skills.update(result.id, {
            tags: values.tags,
            supportedAgents: values.supportedAgents,
          });
        }

        pendingZipBlobRef.current = null;
        setIsAfterUpload(false);
        message.success('创建成功');
      } else if (editingSkill?.id) {
        // 编辑现有记录
        await api.skills.update(editingSkill.id, values);
        message.success('更新成功');
      } else {
        // 手动创建
        await api.skills.create(values);
        message.success('创建成功');
      }
      setModalVisible(false);
      loadSkills();
      loadTags();
    } catch (error: any) {
      message.destroy('uploading');
      const errorMsg = error.message || error.response?.data?.error || '操作失败';
      message.error(errorMsg);
    }
  };

  const handleTagChange = (tag: string, checked: boolean) => {
    const nextSelectedTags = checked ? [tag] : [];
    setSelectedTags(nextSelectedTags);
    setPage(1);
  };

  const handleBuiltInTagClick = (tag: string) => {
    const currentTags = form.getFieldValue('tags') || [];
    if (!currentTags.includes(tag)) {
      form.setFieldsValue({ tags: [...currentTags, tag] });
    }
  };

  const getSourceTypeLabel = (sourceType: SkillSourceType) => {
    const map: Record<string, string> = {
      platform: '平台',
      personal: '个人',
      federated: '联邦',
    };
    return map[sourceType] || sourceType;
  };

  const getSourceTypeColor = (sourceType: SkillSourceType) => {
    const map: Record<string, string> = {
      platform: 'green',
      personal: 'orange',
      federated: 'cyan',
    };
    return map[sourceType] || 'default';
  };

  const tagDropdownRender = (menu: React.ReactNode) => (
    <div>
      {menu}
      <Divider style={{ margin: '8px 0' }} />
      <div style={{ padding: '8px', maxHeight: 200, overflow: 'auto' }}>
        <Text type="secondary" style={{ fontSize: 12, marginBottom: 8, display: 'block' }}>推荐标签：</Text>
        {builtInTagCategories.map(category => (
          <div key={category.name} style={{ marginBottom: 6 }}>
            <Text type="secondary" style={{ fontSize: 11 }}>{category.name}：</Text>
            <div style={{ marginTop: 2 }}>
              {category.tags.map(tag => (
                <Tag
                  key={tag}
                  style={{ cursor: 'pointer', marginBottom: 2, fontSize: 11 }}
                  color="blue"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => handleBuiltInTagClick(tag)}
                >
                  {tag}
                </Tag>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );

  // 上传成功后自动填充表单
  // 目录上传处理
  const handleDirectorySelect = () => {
    directoryInputRef.current?.click();
  };

  // 解析 SKILL.md 内容提取描述
  const parseSkillMD = (content: string): { name: string; description: string } => {
    let name = '';
    let description = '';

    // 1. 首先尝试解析 YAML front matter
    const frontMatterMatch = content.match(/^---\s*\n([\s\S]*?)\n---/);
    if (frontMatterMatch) {
      const frontMatter = frontMatterMatch[1];
      // 提取 description
      const descMatch = frontMatter.match(/description:\s*(.+)/i);
      if (descMatch) {
        description = descMatch[1].trim();
        // 移除引号
        description = description.replace(/^["']|["']$/g, '');
      }
      // 提取 name
      const nameMatch = frontMatter.match(/name:\s*(.+)/i);
      if (nameMatch) {
        name = nameMatch[1].trim();
        // 移除引号
        name = name.replace(/^["']|["']$/g, '');
      }
    }

    // 2. 如果没有从 front matter 获取到 name，尝试从标题获取
    if (!name) {
      const titleMatch = content.match(/^#\s+(.+)$/m);
      if (titleMatch) {
        name = titleMatch[1].trim();
      }
    }

    // 3. 如果没有从 front matter 获取到 description，尝试从 ## Description 获取
    if (!description) {
      const patterns = [
        /##\s*(?:Description|描述)\s*\n+([\s\S]*?)(?=\n##|$)/i,
        /##\s*(?:Description|描述)\s*[:：]?\s*([\s\S]*?)(?=\n##|$)/i,
      ];

      for (const pattern of patterns) {
        const descMatch = content.match(pattern);
        if (descMatch && descMatch[1]) {
          description = descMatch[1].trim();
          break;
        }
      }
    }

    return { name, description };
  };

  // 清理名称格式
  const cleanName = (name: string): string => {
    if (!name) return '';
    // 只保留小写字母、数字、中划线
    let cleaned = name.toLowerCase().replace(/[^a-z0-9-]/g, '-');
    cleaned = cleaned.replace(/^-+|-+$/g, '');
    // 确保以字母开头
    if (cleaned && !/^[a-z]/.test(cleaned)) {
      cleaned = 's-' + cleaned;
    }
    return cleaned;
  };

  const handleDirectoryChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    // 获取目录名
    const firstFile = files[0];
    const pathParts = firstFile.webkitRelativePath.split('/');
    const directoryName = cleanName(pathParts[0]);

    if (!directoryName) {
      message.error('目录名格式无效');
      return;
    }

    // 检查是否包含 SKILL.md
    const skillMdFile = Array.from(files).find(f => {
      const parts = f.webkitRelativePath.split('/');
      return parts[parts.length - 1].toLowerCase() === 'skill.md';
    });

    if (!skillMdFile) {
      message.error('目录中未找到 SKILL.md 文件');
      return;
    }

    // 检查总大小
    const totalSize = Array.from(files).reduce((sum, f) => sum + f.size, 0);
    if (totalSize > 5 * 1024 * 1024) {
      message.error('目录总大小不能超过 5MB');
      return;
    }

    try {
      // 解析 SKILL.md 获取描述
      const skillMdContent = await skillMdFile.text();
      const metadata = parseSkillMD(skillMdContent);

      // 打包 zip（保存起来等用户确认后上传）
      message.loading({ content: '正在解析目录...', key: 'packing' });

      const zip = new JSZip();
      for (const file of Array.from(files)) {
        const parts = file.webkitRelativePath.split('/');
        const relativePath = parts.slice(1).join('/');
        if (relativePath) {
          const content = await file.arrayBuffer();
          zip.file(relativePath, content);
        }
      }

      const zipBlob = await zip.generateAsync({ type: 'blob' });
      pendingZipBlobRef.current = zipBlob;

      message.destroy('packing');

      // 展示表单让用户确认（不创建记录）
      setEditingSkill({
        id: '',
        name: directoryName,
        description: metadata.description,
        sourceType: sourceType,
        isPublic: sourceType !== 'personal', // 个人类型私有，其他类型公开
      } as any);
      setIsAfterUpload(true);
      form.setFieldsValue({
        name: directoryName,
        description: metadata.description || '',
        tags: [],
        sourceType: sourceType,
        supportedAgents: [],
        isPublic: sourceType !== 'personal', // 个人类型私有，其他类型公开
      });
      setModalVisible(true);
    } catch (error: any) {
      message.destroy('packing');
      message.error('解析目录失败');
    }

    e.target.value = '';
  };

  // 仓库导入
  const handleRepoImport = async () => {
    if (!repoUrl.trim()) {
      message.error('请输入仓库地址');
      return;
    }
    setImporting(true);
    try {
      const response = await api.skills.importRepo(repoUrl.trim());
      message.success('仓库导入成功');
      // 后端已创建记录，设置为编辑模式
      setEditingSkill(response);
      form.setFieldsValue({
        name: response.name,
        description: response.description || '',
        tags: response.tags || [],
        sourceType: response.sourceType || 'personal',
        isPublic: response.isPublic !== undefined ? response.isPublic : true,
      });
      setSourceType(response.sourceType || 'personal');
    } catch (error: any) {
      const errorData = error.response?.data;
      message.error(errorData?.error || '仓库导入失败');
    } finally {
      setImporting(false);
    }
  };

  // 联邦源导入
  const handleFederatedImport = async () => {
    if (!selectedRegistryId) {
      message.error('请选择联邦源');
      return;
    }
    setImporting(true);
    try {
      const response = await api.skills.importFederated(selectedRegistryId);
      // 如果返回的是技能对象（已指定技能名称）
      if ('id' in response) {
        message.success('联邦源导入成功');
        setEditingSkill(response);
        form.setFieldsValue({
          name: response.name,
          description: response.description || '',
          tags: response.tags || [],
          sourceType: response.sourceType || 'federated',
          isPublic: response.isPublic !== undefined ? response.isPublic : true,
        });
        setSourceType(response.sourceType || 'federated');
      } else {
        // 返回的是技能列表
        message.info('请选择要导入的技能');
      }
    } catch (error: any) {
      const errorData = error.response?.data;
      message.error(errorData?.error || '联邦源导入失败');
    } finally {
      setImporting(false);
    }
  };

  // 渲染创建方式选择区域
  const renderCreateMethodSelector = () => {
    // 真正的编辑模式（非上传后）不显示
    if (editingSkill && !isAfterUpload) return null;

    return (
      <div style={{ marginBottom: 12, padding: 16, background: 'var(--ant-color-bg-container)', borderRadius: 8, border: '1px solid var(--ant-color-border)' }}>
        <div style={{ marginBottom: 16 }}>
          <Text strong style={{ marginRight: 12 }}>来源：</Text>
          <Radio.Group
            value={sourceType}
            onChange={(e) => {
              setSourceType(e.target.value);
              form.setFieldValue('sourceType', e.target.value);
              // 非个人来源默认公开，不可修改
              if (e.target.value !== 'personal') {
                form.setFieldValue('isPublic', true);
              }
            }}
            disabled={isAfterUpload}
          >
            <Radio value="platform">平台</Radio>
            <Radio value="personal">个人</Radio>
            <Radio value="federated">联邦</Radio>
          </Radio.Group>
        </div>

        {sourceType === 'federated' ? (
          // 联邦源选择或本地上传
          <div>
            <Text type="secondary" style={{ marginBottom: 8, display: 'block' }}>
              从联邦技能源下载或本地上传
            </Text>
            <Radio.Group
              value={createMethod}
              onChange={(e) => setCreateMethod(e.target.value)}
              style={{ marginBottom: 12 }}
              disabled={isAfterUpload}
            >
              <Radio.Button value="upload">
                <CloudUploadOutlined /> 本地上传
              </Radio.Button>
              <Radio.Button value="repo">
                <CloudDownloadOutlined /> 联邦源下载
              </Radio.Button>
            </Radio.Group>

            {createMethod === 'upload' && renderDirectoryUpload()}

            {createMethod === 'repo' && (
              <Space.Compact style={{ width: '100%' }}>
                <Select
                  style={{ width: 'calc(100% - 80px)' }}
                  placeholder="选择联邦技能源"
                  value={selectedRegistryId || undefined}
                  onChange={setSelectedRegistryId}
                  options={registries.map(r => ({ label: r.name, value: r.id }))}
                  disabled={isAfterUpload}
                />
                <Button type="primary" onClick={handleFederatedImport} loading={importing} disabled={isAfterUpload}>
                  导入
                </Button>
              </Space.Compact>
            )}
          </div>
        ) : (
          // 平台/个人的创建方式选择
          <div>
            <Text type="secondary" style={{ marginBottom: 8, display: 'block' }}>
              创建方式
            </Text>
            <Radio.Group
              value={createMethod}
              onChange={(e) => setCreateMethod(e.target.value)}
              style={{ marginBottom: 12 }}
              disabled={isAfterUpload}
            >
              <Radio.Button value="upload">
                <CloudUploadOutlined /> 本地上传
              </Radio.Button>
              <Radio.Button value="repo">
                <CloudDownloadOutlined /> 仓库下载
              </Radio.Button>
            </Radio.Group>

            {createMethod === 'upload' && renderDirectoryUpload()}

            {createMethod === 'repo' && (
              <Space.Compact style={{ width: '100%' }}>
                <Input
                  style={{ width: 'calc(100% - 80px)' }}
                  placeholder="https://github.com/user/skill-repo.git"
                  prefix={<LinkOutlined />}
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  disabled={isAfterUpload}
                />
                <Button type="primary" onClick={handleRepoImport} loading={importing} disabled={isAfterUpload}>
                  导入
                </Button>
              </Space.Compact>
            )}
          </div>
        )}
      </div>
    );
  };

  // 渲染基本属性表单
  const renderBasicForm = () => (
    <Form
      form={form}
      layout="vertical"
      onFinish={handleSubmit}
    >
      <Form.Item name="sourceType" hidden>
        <Input />
      </Form.Item>

      {/* 来源 */}
      <Form.Item label="来源">
        <Radio.Group
          value={sourceType}
          onChange={(e) => {
            setSourceType(e.target.value);
            form.setFieldValue('sourceType', e.target.value);
            // 非个人来源默认公开，不可修改
            if (e.target.value !== 'personal') {
              form.setFieldValue('isPublic', true);
            }
          }}
          disabled={!!editingSkill}
        >
          <Radio value="platform">平台</Radio>
          <Radio value="personal">个人</Radio>
          <Radio value="federated">联邦</Radio>
        </Radio.Group>
      </Form.Item>

      <Form.Item
        name="name"
        label="名称"
        rules={[{ required: true, message: '请输入名称' }]}
        extra="只允许小写字母、数字和中划线，如：java-coding-standards"
      >
        <Input placeholder="java-coding-standards" disabled={!!editingSkill} />
      </Form.Item>

      <Form.Item name="description" label="描述">
        <Input.TextArea rows={3} placeholder="技能描述" />
      </Form.Item>

      <Form.Item label="标签">
        <Form.Item name="tags" noStyle>
          <Select
            mode="tags"
            placeholder="输入标签或从下拉列表选择推荐标签"
            style={{ width: '100%' }}
            dropdownRender={tagDropdownRender}
          />
        </Form.Item>
      </Form.Item>

      
      <Form.Item
        name="supportedAgents"
        label="兼容 Agent"
        rules={[{ required: true, message: '请选择至少一个 Agent 类型' }]}
      >
        <Select
          mode="multiple"
          placeholder="选择兼容的 Agent 类型"
          style={{ width: '100%' }}
          options={agentTypeOptions}
        />
      </Form.Item>

      {/* 可见性 - 始终显示，非个人来源不可编辑 */}
      <Form.Item
        noStyle
        shouldUpdate={(prevValues, currentValues) => prevValues.sourceType !== currentValues.sourceType}
      >
        {({ getFieldValue }) => {
          const isPersonal = getFieldValue('sourceType') === 'personal';
          return (
            <Form.Item name="isPublic" label="可见性">
              <Radio.Group disabled={!isPersonal}>
                <Radio value={false}>私有</Radio>
                <Radio value={true}>公开</Radio>
              </Radio.Group>
            </Form.Item>
          );
        }}
      </Form.Item>
    </Form>
  );

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>Skills</Title>
          <Text type="secondary">管理可复用的技能</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建 Skill
        </Button>
      </div>

      {/* 筛选区域 */}
      <Card style={{ marginBottom: 16 }}>
        <Space wrap size="middle">
          <Space>
            <Text strong>来源：</Text>
            <Select
              value={sourceFilter}
              onChange={(value) => { setSourceFilter(value); setPage(1); }}
              style={{ width: 120 }}
              allowClear
              placeholder="全部来源"
            >
              <Select.Option value="platform">平台</Select.Option>
              <Select.Option value="personal">个人</Select.Option>
              <Select.Option value="federated">联邦</Select.Option>
            </Select>
          </Space>
          <Space>
            <Text strong>Agent：</Text>
            <Select
              value={agentTypeFilter}
              onChange={(value) => { setAgentTypeFilter(value); setPage(1); }}
              style={{ width: 140 }}
              allowClear
              placeholder="全部类型"
            >
              {agentTypeOptions.map(opt => (
                <Select.Option key={opt.value} value={opt.value}>{opt.label}</Select.Option>
              ))}
            </Select>
          </Space>
          {allTags.length > 0 && (
            <Space>
              <Text strong>标签：</Text>
              {allTags.slice(0, 10).map(tag => (
                <CheckableTagAnt
                  key={tag}
                  checked={selectedTags.includes(tag)}
                  onChange={(checked) => handleTagChange(tag, checked)}
                >
                  {tag}
                </CheckableTagAnt>
              ))}
              {allTags.length > 10 && <Text type="secondary">+{allTags.length - 10}</Text>}
            </Space>
          )}
          <Input.Search
            placeholder="搜索 Skills..."
            allowClear
            style={{ width: 250 }}
            onSearch={(value) => { setSearchText(value); setPage(1); }}
          />
        </Space>
      </Card>

      {/* 技能卡片列表 */}
      <Spin spinning={loading}>
        {skills.length === 0 ? (
          <Empty description="暂无 Skills" style={{ padding: 48 }} />
        ) : (
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
            gap: 16,
          }}>
            {skills.map(skill => (
              <Card
                key={skill.id}
                className="skill-card"
                hoverable
                styles={{
                  body: { padding: 12 },
                  actions: { display: 'flex', justifyContent: 'space-around' }
                }}
                title={
                  <div style={{ display: 'flex', alignItems: 'center' }}>
                    <div style={{ flexShrink: 0 }}>
                      <SkillAvatar name={skill.name} />
                    </div>
                    <Tooltip title={skill.name} placement="topLeft">
                      <Text strong style={{ fontSize: 14, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginLeft: 8, flex: 1, minWidth: 0 }}>
                        {skill.name}
                      </Text>
                    </Tooltip>
                  </div>
                }
                extra={
                  <Tag color={getSourceTypeColor(skill.sourceType)} style={{ margin: 0 }}>
                    {getSourceTypeLabel(skill.sourceType)}
                  </Tag>
                }
                actions={[
                  <EyeOutlined key="view" style={{ fontSize: 16 }} onClick={() => message.info('详情功能开发中')} />,
                  <EditOutlined key="edit" style={{ fontSize: 16 }} onClick={() => handleEdit(skill)} />,
                  <Popconfirm
                    key="delete"
                    title="确定要删除这个 Skill 吗？"
                    onConfirm={() => handleDelete(skill.id)}
                    okText="确定"
                    cancelText="取消"
                  >
                    <DeleteOutlined style={{ fontSize: 16, color: '#ff4d4f' }} />
                  </Popconfirm>,
                ]}
              >
                <Paragraph
                  ellipsis={{ rows: 2 }}
                  style={{ marginBottom: 4, fontSize: 13, minHeight: 44, maxHeight: 44 }}
                >
                  {skill.description || '暂无描述'}
                </Paragraph>

                {/* 标签区域 */}
                <div style={{ height: 32, marginBottom: 4, overflow: 'hidden' }}>
                  {skill.tags && skill.tags.length > 0 && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 4, flexWrap: 'nowrap' }}>
                      {skill.tags.slice(0, 2).map(tag => (
                        <Tag key={tag} style={{ fontSize: 11, margin: 0 }}>{tag}</Tag>
                      ))}
                      {skill.tags.length > 2 && (
                        <Tooltip title={skill.tags.slice(2).join(', ')}>
                          <Tag style={{ fontSize: 11, margin: 0, cursor: 'pointer' }}>+{skill.tags.length - 2}</Tag>
                        </Tooltip>
                      )}
                    </div>
                  )}
                </div>

                {/* Agent 区域 */}
                <div style={{ height: 30, marginBottom: 4, overflow: 'hidden' }}>
                  {skill.supportedAgents && skill.supportedAgents.length > 0 && skill.supportedAgents.map(agent => (
                    <Tag key={agent} color="blue" style={{ fontSize: 11, margin: '0 4px 0 0' }}>
                      {agent === 'claude_code' ? 'Claude Code' : 'OpenCode'}
                    </Tag>
                  ))}
                </div>

                <div style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  height: 26,
                  marginTop: 6,
                  paddingTop: 6,
                  borderTop: '1px solid var(--ant-color-border)',
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>已用于</Text>
                    <Text strong style={{ fontSize: 13, color: '#52c41a' }}>{skill.useCount}</Text>
                    <Text type="secondary" style={{ fontSize: 12 }}>个项目</Text>
                  </div>
                  <Tag color={skill.isPublic ? 'blue' : 'default'} style={{ margin: 0, fontSize: 11 }}>
                    {skill.isPublic ? '公开' : '私有'}
                  </Tag>
                </div>
              </Card>
            ))}
          </div>
        )}
      </Spin>

      {/* 分页 */}
      <div style={{ marginTop: 16, display: 'flex', justifyContent: 'center' }}>
        <Pagination
          current={page}
          pageSize={pageSize}
          total={total}
          onChange={(p, ps) => {
            setPage(p);
            setPageSize(ps);
          }}
          showSizeChanger
          showTotal={(t) => `共 ${t} 条`}
          pageSizeOptions={['10', '20', '50']}
        />
      </div>

      {/* 新建/编辑弹窗 */}
      <Modal
        title={editingSkill ? '编辑 Skill' : '新建 Skill'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={700}
        okText="保存"
      >
        {renderCreateMethodSelector()}
        <Divider style={{ margin: '16px 0' }} />
        {renderBasicForm()}
      </Modal>
    </div>
  );
};

export default SkillLibrary;