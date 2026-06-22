import React, { useState, useEffect } from 'react';
import { Button, Input, message, Spin, Typography, Alert } from 'antd';
import { CheckCircleOutlined, EditOutlined } from '@ant-design/icons';
import ConfigSection from '../components/ConfigSection';
import { installApi } from '../../../lib/api';

const { Text } = Typography;
const { TextArea } = Input;

/** 在已有 YAML 文本上把 server: 块下的 port: 替换为新值，保留其它行。
 *  仅做"定位 server: 顶层键 → 改其下首个 port:"的轻量文本替换，避免引入 YAML 库。 */
function patchServerPort(yaml: string, port: number): string {
  const lines = yaml.split('\n');
  let inServer = false;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();
    // 顶层键（无缩进，以 : 结尾）切换 section
    const isTopLevelKey = /^[A-Za-z_][\w-]*:\s*(#.*)?$/.test(line);
    if (isTopLevelKey) {
      inServer = trimmed.startsWith('server:');
      continue;
    }
    if (inServer && /^\s+port:\s*/.test(line)) {
      const indent = line.match(/^\s*/)?.[0] ?? '  ';
      lines[i] = `${indent}port: ${port}`;
      break;
    }
  }
  return lines.join('\n');
}

interface InstallConfig {
  installDir: string;
  createShortcut: boolean;
  launchNow: boolean;
  keepData: boolean;
  dependencies: any[];
  database?: { type: string };
  serverPort?: number;
  configYaml?: string;
}

interface InstalledVersion {
  installed: boolean;
  version?: string;
  installDir?: string;
}

interface SystemConfigProps {
  config: InstallConfig;
  onConfigUpdate: (updates: Partial<InstallConfig>) => void;
  installedVersion?: InstalledVersion;
  isUpgrade?: boolean;
  onValidationChange?: (valid: boolean) => void;
}

const SystemConfig: React.FC<SystemConfigProps> = ({
  config,
  onConfigUpdate,
  installedVersion,
  isUpgrade,
}) => {
  const [loadingConfig, setLoadingConfig] = useState(false);
  const [mergedConfigYaml, setMergedConfigYaml] = useState<string>('');
  const [editingYaml, setEditingYaml] = useState(false);
  const [yamlError, setYamlError] = useState<string | null>(null);
  const [configMerged, setConfigMerged] = useState(false);
  // 用户手动编辑并保存过 YAML 后置为 true，避免被 useEffect 用模板重新生成的内容覆盖
  const [userEditedYaml, setUserEditedYaml] = useState(false);

  // 升级模式或安装目录已有配置时读取已有配置
  useEffect(() => {
    const loadExistingConfig = async () => {
      const targetDir = config.installDir || installedVersion?.installDir;
      if (!targetDir) return;

      setLoadingConfig(true);
      try {
        const result = await installApi.readExistingConfig(targetDir);
        if (result.success && result.config) {
          console.log('[SystemConfig] Loaded existing config from:', targetDir);

          // Parse the returned config - only serverPort is relevant now
          const configData = result.config as any;
          const serverPort = configData?.serverPort || (Array.isArray(configData) ? configData[0] : 26305);

          onConfigUpdate({
            database: { type: 'sqlite' },
            serverPort: serverPort
          });

          if (!isUpgrade) {
            message.info('已加载该目录下的现有配置');
          }
          setConfigMerged(true);
        }
      } catch (e) {
        console.warn('[SystemConfig] Failed to load existing config:', e);
      }
      setLoadingConfig(false);
    };

    if (isUpgrade && installedVersion?.installDir) {
      loadExistingConfig();
    } else if (!isUpgrade && config.installDir) {
      loadExistingConfig();
    }
  }, [isUpgrade, installedVersion, config.installDir]);

  // 生成配置预览 - 当 serverPort 变化时联动更新
  useEffect(() => {
    if (editingYaml) return;

    // 用户已手动编辑过：不再用模板覆盖，只对 server.port 做定点替换，保留其它自定义内容
    if (userEditedYaml) {
      if (!mergedConfigYaml) return;
      const targetPort = config.serverPort || 26305;
      const patched = patchServerPort(mergedConfigYaml, targetPort);
      if (patched !== mergedConfigYaml) {
        setMergedConfigYaml(patched);
        onConfigUpdate({ configYaml: patched });
      }
      return;
    }

    const generatePreview = async () => {
      try {
        const result = await installApi.generateConfigPreview({
          installDir: config.installDir || installedVersion?.installDir,
          database: { type: 'sqlite' },
          serverPort: config.serverPort || 26305
        });

        if (result.success && result.yaml && typeof result.yaml === 'string') {
          setMergedConfigYaml(result.yaml);
          onConfigUpdate({ configYaml: result.yaml });
        } else {
          const errorMsg = result.error || '返回格式错误';
          setMergedConfigYaml(`# 配置生成失败\n# 错误: ${errorMsg}`);
        }
      } catch (e) {
        console.warn('[SystemConfig] Failed to generate config:', e);
        setMergedConfigYaml(`# 配置加载失败\n# 错误: ${e instanceof Error ? e.message : '未知错误'}`);
      }
    };

    generatePreview();
  }, [editingYaml, config.serverPort, config.installDir, installedVersion?.installDir]);

  // YAML 编辑处理
  const handleYamlChange = (value: string) => {
    setMergedConfigYaml(value);
    setYamlError(null);
  };

  const handleSaveYaml = () => {
    try {
      if (mergedConfigYaml.trim().length === 0) {
        setYamlError('配置不能为空');
        return;
      }
      setEditingYaml(false);
      setYamlError(null);
      setUserEditedYaml(true);
      onConfigUpdate({ configYaml: mergedConfigYaml });
      message.success('配置已更新');
    } catch (e) {
      setYamlError('配置格式错误');
    }
  };

  if (loadingConfig) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" tip="加载已有配置..." />
      </div>
    );
  }

  return (
    <div style={{ flex: 1, overflow: 'auto' }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>系统配置</h2>
      <p style={{ color: '#666', marginBottom: 20 }}>
        {isUpgrade ? '已加载现有配置，如需修改请直接调整' : '请配置 Colink 运行所需的参数'}
        {configMerged && isUpgrade && (
          <span style={{ color: '#52c41a', marginLeft: 8 }}>
            <CheckCircleOutlined /> 配置已合并
          </span>
        )}
      </p>

      <ConfigSection title="数据库配置">
        <div style={{
          background: '#f0f5ff',
          border: '1px solid #adc6ff',
          borderRadius: 4,
          padding: 12
        }}>
          <Text type="secondary">
            SQLite 数据库将自动创建在安装目录下：./data/sqlite/colink.db
          </Text>
          <Text type="secondary" style={{ display: 'block', marginTop: 4 }}>
            无需额外配置，首次安装时自动初始化数据库结构。
          </Text>
        </div>
      </ConfigSection>

      <ConfigSection title="服务配置">
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
            服务端口
          </label>
          <Input
            type="number"
            value={config.serverPort || 26305}
            onChange={(e) => {
              onConfigUpdate({ serverPort: parseInt(e.target.value) || 26305 });
              setConfigMerged(false);
            }}
            placeholder="26305"
          />
          <span style={{ fontSize: 12, color: '#999' }}>Web 控制台访问端口</span>
        </div>
      </ConfigSection>

      {/* 完整配置预览 */}
      <ConfigSection title="完整配置预览">
        <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Text type="secondary">
            {isUpgrade && configMerged
              ? '升级时将自动合并您的配置与新模板（您的值优先）'
              : userEditedYaml
                ? '已使用自定义内容，仅服务端口会随上方表单同步'
                : '以下是将生成的完整配置文件内容'
            }
          </Text>
          <div style={{ display: 'flex', gap: 8 }}>
            {editingYaml ? (
              <>
                <Button size="small" onClick={() => {
                  setEditingYaml(false);
                  setYamlError(null);
                }}>
                  取消
                </Button>
                <Button size="small" type="primary" onClick={handleSaveYaml}>
                  保存
                </Button>
              </>
            ) : (
              <Button size="small" icon={<EditOutlined />} onClick={() => setEditingYaml(true)}>
                编辑
              </Button>
            )}
          </div>
        </div>

        {yamlError && (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 12 }}
            message={yamlError}
          />
        )}

        <div style={{
          background: '#fafafa',
          border: '1px solid #e8e8e8',
          borderRadius: 6,
          overflow: 'hidden'
        }}>
          {editingYaml ? (
            <TextArea
              value={mergedConfigYaml}
              onChange={(e) => handleYamlChange(e.target.value)}
              autoSize={{ minRows: 15, maxRows: 25 }}
              style={{
                fontFamily: 'Consolas, Monaco, monospace',
                fontSize: 12,
                lineHeight: 1.5,
                border: 'none',
                background: 'transparent'
              }}
            />
          ) : (
            <pre style={{
              margin: 0,
              padding: 12,
              maxHeight: 400,
              overflow: 'auto',
              fontFamily: 'Consolas, Monaco, monospace',
              fontSize: 12,
              lineHeight: 1.5,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word'
            }}>
              {mergedConfigYaml || '加载中...'}
            </pre>
          )}
        </div>
      </ConfigSection>
    </div>
  );
};

export default SystemConfig;