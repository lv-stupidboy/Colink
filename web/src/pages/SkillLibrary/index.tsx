import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Card, Button, Modal, Form, Input, Select, message, Space, Tag, Typography,
  Popconfirm, Empty, Spin, Divider, Tooltip, Radio, Pagination, Checkbox, List, Table
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
import type { Skill, SkillSourceType, BuiltInTagCategory, SkillRegistry, RemoteSkill, ScanResult, SkillImportItem } from '@/types';

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

  // 联邦源扫描状态
  const [scanModalVisible, setScanModalVisible] = useState(false);
  const [scanLoading, setScanLoading] = useState(false);
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [selectedRemoteSkills, setSelectedRemoteSkills] = useState<RemoteSkill[]>([]);
  const [batchEditMode, setBatchEditMode] = useState(false);
  const [batchSkills, setBatchSkills] = useState<SkillImportItem[]>([]);
  const [batchImporting, setBatchImporting] = useState(false);
  const [unifySettings, setUnifySettings] = useState(false);
  const [unifiedTags, setUnifiedTags] = useState<string[]>([]);
  // 冲突处理状态
  const [conflictModalVisible, setConflictModalVisible] = useState(false);
  const [conflictItems, setConflictItems] = useState<RemoteSkill[]>([]);
  const [conflictChoices, setConflictChoices] = useState<Record<string, 'create' | 'update'>>({});
  const [currentRegistryName, setCurrentRegistryName] = useState('');
  const [unifiedAgents, setUnifiedAgents] = useState<string[]>([]);

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
      } else if (sourceType === 'federated' && selectedRegistryId) {
        // 单选联邦导入：下载并创建 skill 文件
        await api.skills.importFederated(selectedRegistryId, values.name);
        // 更新额外字段（如果有）
        if (values.tags?.length || values.supportedAgents?.length) {
          // 需要先获取刚创建的 skill id
          const skillsList = await api.skills.list({ search: values.name });
          const newSkill = skillsList.data.find(s => s.name === values.name);
          if (newSkill) {
            await api.skills.update(newSkill.id, {
              tags: values.tags,
              supportedAgents: values.supportedAgents,
            });
          }
        }
        message.success('联邦导入成功');
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

  // 联邦源导入（触发扫描）
  const handleFederatedImport = async () => {
    if (!selectedRegistryId) {
      message.error('请选择联邦源');
      return;
    }
    await handleScanFederated();
  };

  // 扫描联邦源
  const handleScanFederated = async () => {
    setScanLoading(true);
    try {
      const result = await api.skills.scanFederatedSkills(selectedRegistryId);
      setScanResult(result);
      setSelectedRemoteSkills([]);
      setScanModalVisible(true);
    } catch (error: any) {
      message.error(error.response?.data?.error || '扫描联邦源失败');
    } finally {
      setScanLoading(false);
    }
  };

  // 冲突分析函数
  const analyzeConflicts = (selectedSkills: RemoteSkill[], registryId: string): {
    autoUpdateItems: RemoteSkill[];
    conflictItems: RemoteSkill[];
    createItems: RemoteSkill[];
  } => {
    const autoUpdateItems: RemoteSkill[] = [];
    const conflictItems: RemoteSkill[] = [];
    const createItems: RemoteSkill[] = [];

    for (const skill of selectedSkills) {
      if (!skill.existsLocally) {
        createItems.push(skill);
      } else if (skill.localSkill?.sourceType === 'federated' &&
                 skill.localSkill.sourceRegistryId === registryId) {
        autoUpdateItems.push(skill);
      } else {
        conflictItems.push(skill);
      }
    }

    return { autoUpdateItems, conflictItems, createItems };
  };

  // 执行导入操作
  const performImport = async (
    autoUpdateItems: RemoteSkill[],
    createItems: RemoteSkill[],
    conflictChoices: Record<string, 'create' | 'update'>
  ) => {
    setBatchImporting(true);
    try {
      const skills: SkillImportItem[] = [];

      // 创建项
      for (const skill of createItems) {
        skills.push({
          name: skill.name,
          path: skill.path,
          description: skill.description,
          tags: [],
          supportedAgents: ['claude_code'],
          importMode: 'create',
        });
      }

      // 自动更新项（同源）
      for (const skill of autoUpdateItems) {
        skills.push({
          name: skill.name,
          path: skill.path,
          description: skill.description,
          tags: [],
          supportedAgents: ['claude_code'],
          importMode: 'update',
          targetSkillId: skill.localSkill?.id,
        });
      }

      // 冲突项（根据用户选择）
      for (const skill of conflictItems) {
        const choice = conflictChoices[skill.name];
        skills.push({
          name: skill.name,
          path: skill.path,
          description: skill.description,
          tags: [],
          supportedAgents: ['claude_code'],
          importMode: choice,
          targetSkillId: choice === 'update' ? skill.localSkill?.id : undefined,
        });
      }

      const result = await api.skills.batchImportFederated({
        registryId: selectedRegistryId,
        skills,
      });

      // 显示导入结果
      const summary = result.conflictSummary;
      let successMsg = `成功导入 ${result.imported.length} 个 Skill`;
      if (result.updated?.length > 0) {
        successMsg += `，更新 ${result.updated.length} 个`;
      }
      if (summary) {
        if (summary.autoUpdated > 0) successMsg += `（自动更新 ${summary.autoUpdated} 个）`;
        if (summary.userCreated > 0) successMsg += `（新建 ${summary.userCreated} 个）`;
        if (summary.userUpdated > 0) successMsg += `（更新 ${summary.userUpdated} 个）`;
      }
      message.success(successMsg);

      if (result.skipped.length > 0) {
        message.warning(`跳过 ${result.skipped.length} 个：${result.skipped.map(s => s.name).join(', ')}`);
      }

      // 关闭所有相关弹窗
      setScanModalVisible(false);
      setConflictModalVisible(false);
      setModalVisible(false);
      setSelectedRemoteSkills([]);
      loadSkills();
    } catch (error: any) {
      message.error(error.response?.data?.error || '导入失败');
    } finally {
      setBatchImporting(false);
    }
  };

  // 确认冲突选择
  const handleConfirmConflict = () => {
    // 检查是否所有冲突项都已选择
    const unselected = conflictItems.filter(s => !conflictChoices[s.name]);
    if (unselected.length > 0) {
      message.error(`以下 Skill 未选择操作：${unselected.map(s => s.name).join(', ')}`);
      return;
    }

    setConflictModalVisible(false);

    // 重新分析冲突（获取 autoUpdateItems 和 createItems）
    const { autoUpdateItems, createItems } = analyzeConflicts(selectedRemoteSkills, selectedRegistryId);

    // 执行导入
    performImport(autoUpdateItems, createItems, conflictChoices);
  };

  // 确认导入选中的 Skill
  const handleConfirmImport = () => {
    if (selectedRemoteSkills.length === 0) {
      message.error('请选择至少一个 Skill');
      return;
    }

    // 分析冲突情况
    const { autoUpdateItems, conflictItems, createItems } = analyzeConflicts(
      selectedRemoteSkills,
      selectedRegistryId
    );

    // 如果没有冲突项，直接导入
    if (conflictItems.length === 0) {
      setScanModalVisible(false);
      performImport(autoUpdateItems, createItems, {});
      return;
    }

    // 有冲突项，显示冲突弹窗
    setConflictItems(conflictItems);
    setConflictChoices({});
    setCurrentRegistryName(scanResult?.registryName || '');
    setScanModalVisible(false);
    setConflictModalVisible(true);
  };

  // 批量保存 Skill
  const handleBatchSave = async () => {
    // 验证所有 Skill 的 supportedAgents
    const invalidSkills = batchSkills.filter(s => s.supportedAgents.length === 0);
    if (!unifySettings && invalidSkills.length > 0) {
      message.error(`以下 Skill 未设置 Agent：${invalidSkills.map(s => s.name).join(', ')}`);
      return;
    }
    if (unifySettings && unifiedAgents.length === 0) {
      message.error('请设置统一 Agent');
      return;
    }

    setBatchImporting(true);
    try {
      const finalSkills = batchSkills.map(s => ({
        ...s,
        tags: unifySettings ? [...s.tags, ...unifiedTags] : s.tags,
        supportedAgents: unifySettings ? unifiedAgents : s.supportedAgents,
      }));

      const result = await api.skills.batchImportFederated({
        registryId: selectedRegistryId,
        skills: finalSkills,
      });

      message.success(`成功导入 ${result.imported.length} 个 Skill`);
      if (result.skipped.length > 0) {
        message.warning(`跳过 ${result.skipped.length} 个：${result.skipped.map(s => s.name).join(', ')}`);
      }

      setModalVisible(false);
      setBatchEditMode(false);
      setBatchSkills([]);
      setSelectedRemoteSkills([]);
      loadSkills();
    } catch (error: any) {
      message.error(error.response?.data?.error || '批量导入失败');
    } finally {
      setBatchImporting(false);
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
                <Button type="primary" onClick={handleFederatedImport} loading={scanLoading || importing} disabled={isAfterUpload}>
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
          <Title level={2} style={{ margin: 0 }}>Skills管理</Title>
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

                {/* 路径区域 - 仅联邦类型显示 */}
                {skill.sourceType === 'federated' && skill.sourcePath && (
                  <div style={{ marginBottom: 4 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      路径: {skill.sourcePath}
                    </Text>
                  </div>
                )}

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
        title={batchEditMode ? `批量导入 (${batchSkills.length} 个 Skill)` : (editingSkill ? '编辑 Skill' : '新建 Skill')}
        open={modalVisible}
        onOk={() => batchEditMode ? handleBatchSave() : form.submit()}
        onCancel={() => {
          setModalVisible(false);
          setBatchEditMode(false);
          setBatchSkills([]);
        }}
        width={batchEditMode ? 900 : 700}
        okText={batchEditMode ? '保存全部' : '保存'}
        confirmLoading={batchImporting}
      >
        {batchEditMode ? (
          // 批量编辑模式
          <>
            <Text strong style={{ marginBottom: 12, display: 'block' }}>
              已选择 {batchSkills.length} 个 Skill，请补充属性信息：
            </Text>

            {/* 统一设置 */}
            <div style={{ marginBottom: 16, padding: 12, background: 'var(--ant-color-bg-container)', borderRadius: 8 }}>
              <Checkbox
                checked={unifySettings}
                onChange={(e) => setUnifySettings(e.target.checked)}
              >
                统一设置：为所有 Skill 应用相同的标签和 Agent
              </Checkbox>
              {unifySettings && (
                <Space style={{ marginTop: 8, width: '100%' }} direction="vertical">
                  <Select
                    mode="tags"
                    placeholder="统一标签"
                    style={{ width: '100%' }}
                    value={unifiedTags}
                    onChange={setUnifiedTags}
                  />
                  <Select
                    mode="multiple"
                    placeholder="统一 Agent（必填）"
                    style={{ width: '100%' }}
                    options={agentTypeOptions}
                    value={unifiedAgents}
                    onChange={setUnifiedAgents}
                  />
                </Space>
              )}
            </div>

            {/* Skill 表格 */}
            <Table
              dataSource={batchSkills}
              columns={[
                {
                  title: '名称',
                  dataIndex: 'name',
                  key: 'name',
                  width: 150,
                  render: (name: string, _record, index) => (
                    <Input
                      value={name}
                      onChange={(e) => {
                        const updated = [...batchSkills];
                        updated[index].name = e.target.value;
                        setBatchSkills(updated);
                      }}
                    />
                  ),
                },
                {
                  title: '描述',
                  dataIndex: 'description',
                  key: 'description',
                  render: (desc: string, _record, index) => (
                    <Input
                      value={desc}
                      onChange={(e) => {
                        const updated = [...batchSkills];
                        updated[index].description = e.target.value;
                        setBatchSkills(updated);
                      }}
                    />
                  ),
                },
                {
                  title: '标签',
                  dataIndex: 'tags',
                  key: 'tags',
                  width: 150,
                  render: (_, _record, index) => (
                    unifySettings ? (
                      <Text type="secondary">使用统一设置</Text>
                    ) : (
                      <Select
                        mode="tags"
                        placeholder="标签"
                        style={{ width: '100%' }}
                        value={batchSkills[index].tags}
                        onChange={(val) => {
                          const updated = [...batchSkills];
                          updated[index].tags = val;
                          setBatchSkills(updated);
                        }}
                      />
                    )
                  ),
                },
                {
                  title: 'Agent',
                  dataIndex: 'supportedAgents',
                  key: 'supportedAgents',
                  width: 150,
                  render: (_, _record, index) => (
                    unifySettings ? (
                      <Text type="secondary">使用统一设置</Text>
                    ) : (
                      <Select
                        mode="multiple"
                        placeholder="Agent（必填）"
                        style={{ width: '100%' }}
                        options={agentTypeOptions}
                        value={batchSkills[index].supportedAgents}
                        onChange={(val) => {
                          const updated = [...batchSkills];
                          updated[index].supportedAgents = val;
                          setBatchSkills(updated);
                        }}
                      />
                    )
                  ),
                },
              ]}
              rowKey="name"
              pagination={false}
              size="small"
              scroll={{ y: 300 }}
            />
          </>
        ) : (
          // 单个编辑模式
          <>
            {renderCreateMethodSelector()}
            <Divider style={{ margin: '16px 0' }} />
            {renderBasicForm()}
          </>
        )}
      </Modal>

      {/* 联邦源扫描弹窗 */}
      <Modal
        title="从联邦源导入 Skill"
        open={scanModalVisible}
        onCancel={() => setScanModalVisible(false)}
        width={600}
        footer={[
          <Button key="cancel" onClick={() => setScanModalVisible(false)}>取消</Button>,
          <Button key="confirm" type="primary" onClick={handleConfirmImport} disabled={selectedRemoteSkills.length === 0}>
            确认导入（已选择 {selectedRemoteSkills.length} 个）
          </Button>,
        ]}
      >
        {scanResult && (
          <>
            <div style={{ marginBottom: 12 }}>
              <Text strong>联邦源：</Text>{scanResult.registryName}
              <br />
              <Text type="secondary">{scanResult.registryUrl}</Text>
            </div>
            <List
              dataSource={scanResult.skills}
              renderItem={(skill) => (
                <List.Item
                >
                  <Checkbox
                    checked={selectedRemoteSkills.some(s => s.name === skill.name)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        setSelectedRemoteSkills([...selectedRemoteSkills, skill]);
                      } else {
                        setSelectedRemoteSkills(selectedRemoteSkills.filter(s => s.name !== skill.name));
                      }
                    }}
                  >
                    <div>
                      <Text strong>{skill.name}</Text>
                      {skill.existsLocally && skill.localSkill && (
                        <Tag color={getSourceTypeColor(skill.localSkill.sourceType as SkillSourceType)} style={{ marginLeft: 8 }}>
                          来自: {skill.localSkill.sourceRegistryName || getSourceTypeLabel(skill.localSkill.sourceType as SkillSourceType)}
                        </Tag>
                      )}
                      <br />
                      <Text type="secondary" style={{ fontSize: 12 }}>{skill.description || '暂无描述'}</Text>
                    </div>
                  </Checkbox>
                </List.Item>
              )}
            />
          </>
        )}
      </Modal>

      {/* 冲突选择弹窗 */}
      <Modal
        title="导入冲突处理"
        open={conflictModalVisible}
        onCancel={() => setConflictModalVisible(false)}
        width={800}
        footer={[
          <Button key="cancel" onClick={() => setConflictModalVisible(false)}>取消</Button>,
          <Button key="all-create" onClick={() => {
            const choices: Record<string, 'create'> = {};
            conflictItems.forEach(s => choices[s.name] = 'create');
            setConflictChoices(choices);
          }}>
            全部新建
          </Button>,
          <Button key="all-update" type="primary" onClick={() => {
            const choices: Record<string, 'update'> = {};
            conflictItems.forEach(s => choices[s.name] = 'update');
            setConflictChoices(choices);
          }}>
            全部更新
          </Button>,
          <Button key="confirm" type="primary" onClick={handleConfirmConflict}>
            确认导入
          </Button>,
        ]}
      >
        <Text type="secondary" style={{ marginBottom: 16, display: 'block' }}>
          以下 Skill 与本地已有同名 Skill 来源不同，请选择处理方式：
        </Text>
        <Table
          dataSource={conflictItems}
          columns={[
            {
              title: '名称',
              dataIndex: 'name',
              key: 'name',
              width: 120,
            },
            {
              title: '路径',
              dataIndex: 'path',
              key: 'path',
              width: 150,
              render: (path: string) => <Text type="secondary" style={{ fontSize: 12 }}>{path || '-'}</Text>,
            },
            {
              title: '本地来源',
              key: 'localSource',
              width: 120,
              render: (_, record) => {
                if (!record.localSkill) return '未知';
                const sourceType = record.localSkill.sourceType as SkillSourceType;
                return (
                  <Tag color={getSourceTypeColor(sourceType)}>
                    {record.localSkill.sourceRegistryName || getSourceTypeLabel(sourceType)}
                  </Tag>
                );
              },
            },
            {
              title: '远程来源',
              key: 'remoteSource',
              width: 120,
              render: () => (
                <Tag color="cyan">{currentRegistryName}</Tag>
              ),
            },
            {
              title: '本地描述',
              key: 'localDesc',
              width: 200,
              render: (_, record) => (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {record.localSkill?.description?.slice(0, 50) || '暂无'}
                  {record.localSkill?.description?.length > 50 ? '...' : ''}
                </Text>
              ),
            },
            {
              title: '远程描述',
              dataIndex: 'description',
              key: 'remoteDesc',
              width: 200,
              render: (desc: string) => (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {desc?.slice(0, 50) || '暂无'}
                  {desc?.length > 50 ? '...' : ''}
                </Text>
              ),
            },
            {
              title: '操作',
              key: 'action',
              width: 150,
              render: (_, record) => (
                <Radio.Group
                  value={conflictChoices[record.name]}
                  onChange={(e) => {
                    setConflictChoices(prev => ({
                      ...prev,
                      [record.name]: e.target.value,
                    }));
                  }}
                >
                  <Radio value="create">新建</Radio>
                  <Radio value="update">更新</Radio>
                </Radio.Group>
              ),
            },
          ]}
          rowKey="name"
          pagination={false}
          size="small"
        />
      </Modal>
    </div>
  );
};

export default SkillLibrary;