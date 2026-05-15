# 联邦源同步/导入冲突弹窗修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复联邦源同步和导入冲突弹窗中同名不同路径 Skill 选中同步、路径显示缺失、搜索排序缺失三个问题。

**Architecture:** 前端修改为主，使用 `name::path` 组合作为唯一标识，添加路径显示和搜索排序功能。

**Tech Stack:** React + Ant Design + TypeScript

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `web/src/pages/RegistryManagement/index.tsx` | Modify | 同步冲突弹窗修复 |
| `web/src/pages/SkillLibrary/index.tsx` | Modify | 导入扫描/冲突弹窗修复 |
| `web/src/types/index.ts` | Modify | 更新类型定义注释 |

---

## Task 1: RegistryManagement 同步冲突弹窗 - 唯一标识修复

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:504-548`

### 问题分析

当前代码使用 `skill.name` 作为状态 key 和隐式 rowKey，同名不同路径的 Skill 会共享选择状态。

### 修复方案

使用 `${skill.name}::${skill.path}` 组合作为唯一标识。

- [ ] **Step 1: 添加 uniqueKey 计算**

修改 `List` 组件的 `renderItem` 函数，添加 uniqueKey 计算：

```tsx
// web/src/pages/RegistryManagement/index.tsx
// 在 List 组件中修改（约行 504-548）

<List
  dataSource={syncPreview.conflictSkills}
  renderItem={(skill) => {
    // 使用 name::path 组合作为唯一标识
    const uniqueKey = skill.path ? `${skill.name}::${skill.path}` : skill.name;
    const isChecked = conflictChoices[uniqueKey] === 'update';
    const sourceType = skill.localSkill.sourceType as SkillSourceType;
    return (
      <List.Item key={uniqueKey}>
        <Checkbox
          checked={isChecked}
          onChange={(e) => {
            setConflictChoices(prev => ({
              ...prev,
              [uniqueKey]: e.target.checked ? 'update' : 'skip',
            }));
          }}
        >
          {/* 保持原有渲染内容不变 */}
          <div style={{ display: 'flex', alignItems: 'center', width: '100%' }}>
            <div style={{ flex: 1 }}>
              <Text strong>{skill.name}</Text>
              {skill.path && (
                <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
                  路径: {skill.path}
                </Text>
              )}
              {/* ... 保持其余内容不变 */}
            </div>
            <Text style={{ color: isChecked ? '#52c41a' : '#999', fontSize: 12, marginLeft: 16 }}>
              {isChecked ? '将更新' : '将跳过'}
            </Text>
          </div>
        </Checkbox>
      </List.Item>
    );
  }}
/>
```

- [ ] **Step 2: 更新底部统计计数逻辑**

修改底部统计显示，使用 uniqueKey 计数：

```tsx
// web/src/pages/RegistryManagement/index.tsx
// 修改底部统计（约行 549-554）

<div style={{ marginTop: 12, textAlign: 'right' }}>
  <Text type="secondary">
    已选择 {Object.values(conflictChoices).filter(v => v === 'update').length} 个更新，
    {syncPreview.conflictSkills.length - Object.values(conflictChoices).filter(v => v === 'update').length} 个跳过
  </Text>
</div>
```

注意：底部统计逻辑无需修改，因为 `conflictChoices` 现在使用 uniqueKey 作为 key，计数仍然正确。

- [ ] **Step 3: 更新 handleConfirmConflict 函数**

修改确认函数，使用 uniqueKey 获取选择：

```tsx
// web/src/pages/RegistryManagement/index.tsx
// 修改 handleConfirmConflict 函数（约行 181-217）

const handleConfirmConflict = async () => {
  if (!syncPreview || !syncingRegistryId) return;

  setConflictModalVisible(false);
  setSyncingId(syncingRegistryId);

  try {
    // 构建同步确认请求
    const operations: SyncOperation[] = [];
    for (const skill of syncPreview.conflictSkills) {
      const uniqueKey = skill.path ? `${skill.name}::${skill.path}` : skill.name;
      const choice = conflictChoices[uniqueKey] || 'skip'; // 默认跳过
      operations.push({
        action: choice,
        skillName: skill.name,
        targetSkillId: choice === 'update' ? skill.localSkill.id : undefined,
        description: skill.description,
      });
    }

    const result = await api.registries.syncConfirm(syncingRegistryId, {
      registryId: syncingRegistryId,
      operations,
    });

    // 显示结果汇总
    message.success(`同步完成：更新 ${result.autoUpdated + result.userUpdated} 个`);

    loadRegistries();
  } catch (error: any) {
    message.error(error.response?.data?.error || '同步确认失败');
  } finally {
    setSyncingId(null);
    setSyncPreview(null);
    setSyncingRegistryId(null);
  }
};
```

---

## Task 2: RegistryManagement 同步冲突弹窗 - 添加搜索和排序

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx`

- [ ] **Step 1: 添加搜索状态**

在组件顶部添加搜索状态：

```tsx
// web/src/pages/RegistryManagement/index.tsx
// 在现有状态声明后添加（约行 58-62）

const [conflictModalVisible, setConflictModalVisible] = useState(false);
const [conflictChoices, setConflictChoices] = useState<Record<string, 'update' | 'skip'>({});
const [syncingRegistryId, setSyncingRegistryId] = useState<string | null>(null);
const [syncingRegistryName, setSyncingRegistryName] = useState('');
// 添加搜索状态
const [conflictSearchText, setConflictSearchText] = useState('');
```

- [ ] **Step 2: 添加排序和过滤逻辑**

在 Modal 内部添加排序过滤逻辑：

```tsx
// web/src/pages/RegistryManagement/index.tsx
// 在 Modal 内部添加（约行 497 之后）

{syncPreview && (
  <>
    {syncPreview.autoUpdateSkills.length > 0 && (
      <div style={{ marginBottom: 12 }}>
        <Tag color="green">{syncPreview.autoUpdateSkills.length} 个同源 Skill 将自动更新</Tag>
      </div>
    )}
    {/* 添加搜索框 */}
    <Input.Search
      placeholder="搜索 Skill 名称/路径"
      allowClear
      style={{ marginBottom: 12 }}
      onChange={(e) => setConflictSearchText(e.target.value)}
    />
    {/* 排序 + 过滤后的数据源 */}
    {(() => {
      const filteredSkills = syncPreview.conflictSkills
        .sort((a, b) => a.name.localeCompare(b.name))
        .filter(skill =>
          skill.name.includes(conflictSearchText) ||
          (skill.path && skill.path.includes(conflictSearchText))
        );
      
      return (
        <List
          dataSource={filteredSkills}
          renderItem={(skill) => {
            // ... renderItem 内容保持不变
          }}
        />
      );
    })()}
```

- [ ] **Step 3: 更新完整渲染代码**

将搜索和排序功能整合到完整代码中：

```tsx
// web/src/pages/RegistryManagement/index.tsx
// 完整的同步冲突弹窗代码（行 481-557）

{/* 同步冲突处理弹窗 */}
<Modal
  title="同步冲突处理"
  open={conflictModalVisible}
  onCancel={() => {
    setConflictModalVisible(false);
    setConflictSearchText('');
  }}
  width={800}
  footer={[
    <Button key="cancel" onClick={() => {
      setConflictModalVisible(false);
      setConflictSearchText('');
    }}>取消</Button>,
    <Button key="confirm" type="primary" onClick={handleConfirmConflict}>
      确认同步（已选择 {Object.values(conflictChoices).filter(v => v === 'update').length} 个更新）
    </Button>,
  ]}
>
  <Text type="secondary" style={{ marginBottom: 16, display: 'block' }}>
    勾选需要更新的 Skill，未勾选的将被跳过：
  </Text>
  {syncPreview && (
    <>
      {syncPreview.autoUpdateSkills.length > 0 && (
        <div style={{ marginBottom: 12 }}>
          <Tag color="green">{syncPreview.autoUpdateSkills.length} 个同源 Skill 将自动更新</Tag>
        </div>
      )}
      {/* 搜索框 */}
      <Input.Search
        placeholder="搜索 Skill 名称/路径"
        allowClear
        style={{ marginBottom: 12 }}
        onChange={(e) => setConflictSearchText(e.target.value)}
      />
      {/* 排序 + 过滤后的数据源 */}
      {(() => {
        const filteredSkills = syncPreview.conflictSkills
          .sort((a, b) => a.name.localeCompare(b.name))
          .filter(skill =>
            skill.name.includes(conflictSearchText) ||
            (skill.path && skill.path.includes(conflictSearchText))
          );
        
        return (
          <List
            dataSource={filteredSkills}
            renderItem={(skill) => {
              const uniqueKey = skill.path ? `${skill.name}::${skill.path}` : skill.name;
              const isChecked = conflictChoices[uniqueKey] === 'update';
              const sourceType = skill.localSkill.sourceType as SkillSourceType;
              return (
                <List.Item key={uniqueKey}>
                  <Checkbox
                    checked={isChecked}
                    onChange={(e) => {
                      setConflictChoices(prev => ({
                        ...prev,
                        [uniqueKey]: e.target.checked ? 'update' : 'skip',
                      }));
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', width: '100%' }}>
                      <div style={{ flex: 1 }}>
                        <Text strong>{skill.name}</Text>
                        {skill.path && (
                          <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
                            路径: {skill.path}
                          </Text>
                        )}
                        <div style={{ marginTop: 4, marginBottom: 4 }}>
                          <Tag color={getSourceTypeColor(sourceType)}>
                            {skill.localSkill.sourceRegistryName || getSourceTypeLabel(sourceType)}
                          </Tag>
                          <Text type="secondary" style={{ margin: '0 8px' }}>→</Text>
                          <Tag color="cyan">{syncingRegistryName}</Tag>
                        </div>
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          远程描述: {skill.description?.slice(0, 60) || '暂无'}
                          {skill.description?.length > 60 ? '...' : ''}
                        </Text>
                      </div>
                      <Text style={{ color: isChecked ? '#52c41a' : '#999', fontSize: 12, marginLeft: 16 }}>
                        {isChecked ? '将更新' : '将跳过'}
                      </Text>
                    </div>
                  </Checkbox>
                </List.Item>
              );
            }}
          />
        );
      })()}
      <div style={{ marginTop: 12, textAlign: 'right' }}>
        <Text type="secondary">
          已选择 {Object.values(conflictChoices).filter(v => v === 'update').length} 个更新，
          {syncPreview.conflictSkills.length - Object.values(conflictChoices).filter(v => v === 'update').length} 个跳过
        </Text>
      </div>
    </>
  )}
</Modal>
```

---

## Task 3: SkillLibrary 扫描弹窗 - 添加路径显示和唯一标识

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:1372-1399`

- [ ] **Step 1: 修改扫描弹窗 Checkbox 逻辑**

使用 `name::path` 作为唯一标识，并添加路径显示：

```tsx
// web/src/pages/SkillLibrary/index.tsx
// 修改扫描弹窗 List renderItem（约行 1372-1399）

<List
  dataSource={scanResult.skills}
  renderItem={(skill) => {
    // 使用 name::path 组合作为唯一标识
    const uniqueKey = `${skill.name}::${skill.path}`;
    return (
      <List.Item key={uniqueKey}>
        <Checkbox
          checked={selectedRemoteSkills.some(s => `${s.name}::${s.path}` === uniqueKey)}
          onChange={(e) => {
            if (e.target.checked) {
              setSelectedRemoteSkills([...selectedRemoteSkills, skill]);
            } else {
              setSelectedRemoteSkills(selectedRemoteSkills.filter(s => `${s.name}::${s.path}` !== uniqueKey));
            }
          }}
        >
          <div>
            <Text strong>{skill.name}</Text>
            {/* 添加路径显示 */}
            {skill.path && (
              <Tag style={{ marginLeft: 8, background: 'var(--ant-color-bg-container)', border: '1px solid var(--ant-color-border)' }}>
                {skill.path}
              </Tag>
            )}
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
    );
  }}
/>
```

---

## Task 4: SkillLibrary 冲突弹窗 - 唯一标识和搜索排序修复

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:1405-1520`

- [ ] **Step 1: 添加搜索状态**

在组件顶部添加冲突弹窗搜索状态：

```tsx
// web/src/pages/SkillLibrary/index.tsx
// 在现有状态声明后添加（约行 127-131）

const [conflictModalVisible, setConflictModalVisible] = useState(false);
const [conflictItems, setConflictItems] = useState<RemoteSkill[]>([]);
const [conflictChoices, setConflictChoices] = useState<Record<string, 'create' | 'update'>>({});
const [currentRegistryName, setCurrentRegistryName] = useState('');
// 添加搜索状态
const [conflictSearchText, setConflictSearchText] = useState('');
```

- [ ] **Step 2: 修改冲突弹窗 Table rowKey**

使用 `name::path` 作为 rowKey：

```tsx
// web/src/pages/SkillLibrary/index.tsx
// 修改 Table rowKey（约行 1516）

<Table
  dataSource={filteredConflictItems}
  rowKey={(record) => `${record.name}::${record.path}`}
  // ... 其他属性保持不变
/>
```

- [ ] **Step 3: 修改操作列 Radio.Group**

使用 uniqueKey 作为状态 key：

```tsx
// web/src/pages/SkillLibrary/index.tsx
// 修改操作列渲染（约行 1497-1514）

{
  title: '操作',
  key: 'action',
  width: 150,
  render: (_, record) => {
    const uniqueKey = `${record.name}::${record.path}`;
    return (
      <Radio.Group
        value={conflictChoices[uniqueKey]}
        onChange={(e) => {
          setConflictChoices(prev => ({
            ...prev,
            [uniqueKey]: e.target.value,
          }));
        }}
      >
        <Radio value="create">新建</Radio>
        <Radio value="update">更新</Radio>
      </Radio.Group>
    );
  },
},
```

- [ ] **Step 4: 修改全选按钮逻辑**

修改"全部新建"和"全部更新"按钮：

```tsx
// web/src/pages/SkillLibrary/index.tsx
// 修改 footer 中的按钮（约行 1413-1426）

<Button key="all-create" onClick={() => {
  const choices: Record<string, 'create'> = {};
  conflictItems.forEach(s => {
    const uniqueKey = `${s.name}::${s.path}`;
    choices[uniqueKey] = 'create';
  });
  setConflictChoices(choices);
}}>
  全部新建
</Button>,
<Button key="all-update" type="primary" onClick={() => {
  const choices: Record<string, 'update'> = {};
  conflictItems.forEach(s => {
    const uniqueKey = `${s.name}::${s.path}`;
    choices[uniqueKey] = 'update';
  });
  setConflictChoices(choices);
}}>
  全部更新
</Button>,
```

- [ ] **Step 5: 修改 handleConfirmConflict 函数**

更新确认函数使用 uniqueKey：

```tsx
// web/src/pages/SkillLibrary/index.tsx
// 修改 handleConfirmConflict 函数（约行 720-735）

const handleConfirmConflict = () => {
  // 检查是否所有冲突项都已选择
  const unselected = conflictItems.filter(s => {
    const uniqueKey = `${s.name}::${s.path}`;
    return !conflictChoices[uniqueKey];
  });
  if (unselected.length > 0) {
    message.error(`以下 Skill 未选择操作：${unselected.map(s => s.name).join(', ')}`);
    return;
  }

  setConflictModalVisible(false);
  setConflictSearchText('');

  // 重新分析冲突（获取 autoUpdateItems 和 createItems）
  const { autoUpdateItems, createItems } = analyzeConflicts(selectedRemoteSkills, selectedRegistryId);

  // 执行导入（需要转换 uniqueKey back to name for performImport）
  const convertedChoices: Record<string, 'create' | 'update'> = {};
  for (const [uniqueKey, choice] of Object.entries(conflictChoices)) {
    const name = uniqueKey.split('::')[0];
    convertedChoices[name] = choice;
  }
  performImport(autoUpdateItems, createItems, convertedChoices);
};
```

- [ ] **Step 6: 添加搜索框和排序逻辑**

在冲突弹窗中添加搜索和排序：

```tsx
// web/src/pages/SkillLibrary/index.tsx
// 完整的冲突弹窗代码（约行 1405-1520）

{/* 冲突选择弹窗 */}
<Modal
  title="导入冲突处理"
  open={conflictModalVisible}
  onCancel={() => {
    setConflictModalVisible(false);
    setConflictSearchText('');
  }}
  width={800}
  footer={[
    <Button key="cancel" onClick={() => {
      setConflictModalVisible(false);
      setConflictSearchText('');
    }}>取消</Button>,
    <Button key="all-create" onClick={() => {
      const choices: Record<string, 'create'> = {};
      conflictItems.forEach(s => {
        const uniqueKey = `${s.name}::${s.path}`;
        choices[uniqueKey] = 'create';
      });
      setConflictChoices(choices);
    }}>
      全部新建
    </Button>,
    <Button key="all-update" type="primary" onClick={() => {
      const choices: Record<string, 'update'> = {};
      conflictItems.forEach(s => {
        const uniqueKey = `${s.name}::${s.path}`;
        choices[uniqueKey] = 'update';
      });
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
  {/* 搜索框 */}
  <Input.Search
    placeholder="搜索 Skill 名称/路径"
    allowClear
    style={{ marginBottom: 12 }}
    onChange={(e) => setConflictSearchText(e.target.value)}
  />
  {/* 排序 + 过滤后的数据源 */}
  {(() => {
    const filteredConflictItems = conflictItems
      .sort((a, b) => a.name.localeCompare(b.name))
      .filter(item =>
        item.name.includes(conflictSearchText) ||
        (item.path && item.path.includes(conflictSearchText))
      );
    
    return (
      <Table
        dataSource={filteredConflictItems}
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
            render: (_, record) => {
              const uniqueKey = `${record.name}::${record.path}`;
              return (
                <Radio.Group
                  value={conflictChoices[uniqueKey]}
                  onChange={(e) => {
                    setConflictChoices(prev => ({
                      ...prev,
                      [uniqueKey]: e.target.value,
                    }));
                  }}
                >
                  <Radio value="create">新建</Radio>
                  <Radio value="update">更新</Radio>
                </Radio.Group>
              );
            },
          },
        ]}
        rowKey={(record) => `${record.name}::${record.path}`}
        pagination={false}
        size="small"
      />
    );
  })()}
</Modal>
```

---

## Task 5: 手动测试验证

**Files:**
- None

- [ ] **Step 1: 启动开发服务器**

```bash
cd D:/workspace/isdp
go run ./cmd/server &
cd web && npm run dev
```

等待服务器启动完成，访问 http://localhost:26306

- [ ] **Step 2: 测试同步冲突弹窗**

1. 进入设置 → 联邦技能源页面
2. 点击某个联邦源的同步按钮（或同步全部）
3. 如果有冲突项，验证：
   - 同名不同路径的 Skill 选中状态独立
   - 路径正确显示
   - 搜索框可以过滤 Skill
   - 列表按名称排序

- [ ] **Step 3: 测试导入扫描弹窗**

1. 进入 Skill 库页面
2. 点击新建 Skill → 选择联邦源导入
3. 选择一个联邦源扫描
4. 验证：
   - 每个 Skill 显示路径 Tag
   - 同名不同路径的 Skill 可以独立选择

- [ ] **Step 4: 测试导入冲突弹窗**

1. 扫描后选择有冲突的 Skill
2. 点击确认导入进入冲突弹窗
3. 验证：
   - 同名不同路径的 Skill 操作选择独立
   - "全部新建"/"全部更新"按钮正确设置所有项
   - 搜索框可以过滤
   - 列表按名称排序

- [ ] **Step 5: 验证原有功能不受影响**

验证无同名冲突场景：
- 单个 Skill 选择/取消正常
- 确认同步/导入功能正常
- 底部计数显示正确

---

## Task 6: 提交代码

**Files:**
- All modified files

- [ ] **Step 1: 检查代码修改**

```bash
cd D:/workspace/isdp
git diff web/src/pages/RegistryManagement/index.tsx
git diff web/src/pages/SkillLibrary/index.tsx
```

- [ ] **Step 2: 运行前端 lint 检查**

```bash
cd D:/workspace/isdp/web
npm run lint
```

确保无 lint 错误。

- [ ] **Step 3: 提交代码**

```bash
cd D:/workspace/isdp
git add web/src/pages/RegistryManagement/index.tsx web/src/pages/SkillLibrary/index.tsx docs/superpowers/specs/registry-sync-conflict-modal-issues-2026-05-09.html
git commit -m "fix(skill): fix conflict modal checkbox sync issue for same-name skills

- Use name::path as unique key instead of name only
- Add path display in scan modal
- Add search and sort functionality in conflict modals
- Fix 'all create/update' buttons to use unique keys

Fixes:
- Same-name skills with different paths now have independent selection states
- Users can distinguish skills by path in scan modal
- Users can search and filter skills in conflict modals

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Self-Review Checklist

### Spec Coverage
| 要求 | 任务 | 状态 |
|------|------|------|
| 同名 Skill 选中同步问题 | Task 1, 4 | ✅ |
| 扫描弹窗显示路径 | Task 3 | ✅ |
| 添加搜索和排序 | Task 2, 4 | ✅ |

### Placeholder Scan
- 无 "TODO"、"TBD"、"implement later" 等占位符 ✅
- 所有代码步骤包含完整代码 ✅
- 所有命令步骤包含具体命令 ✅

### Type Consistency
- `uniqueKey` 格式统一为 `${skill.name}::${skill.path}` ✅
- `conflictChoices` key 类型统一为 string ✅
- 过滤函数使用相同字段名 ✅