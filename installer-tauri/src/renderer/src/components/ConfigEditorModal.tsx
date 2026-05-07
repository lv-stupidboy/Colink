import React, { useState, useEffect } from 'react';
import { Modal, Button, message, Alert, Spin } from 'antd';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { configApi, serviceApi } from '../../../lib/api';

interface ConfigEditorModalProps {
  open: boolean;
  onCancel: () => void;
  onRestartRequired?: () => void;
}

const ConfigEditorModal: React.FC<ConfigEditorModalProps> = ({
  open,
  onCancel,
  onRestartRequired,
}) => {
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [yamlContent, setYamlContent] = useState('');
  const [yamlError, setYamlError] = useState<string | null>(null);
  const [restartModalVisible, setRestartModalVisible] = useState(false);

  // Load config file when modal opens
  useEffect(() => {
    if (open) {
      loadConfig();
    }
  }, [open]);

  const loadConfig = async () => {
    setLoading(true);
    setYamlError(null);
    try {
      const result = await configApi.readConfigFile();
      if (result.success && result.content) {
        setYamlContent(result.content);
      } else {
        setYamlError(result.error || 'Failed to read config file');
      }
    } catch (err) {
      setYamlError(err instanceof Error ? err.message : 'Failed to read config file');
    } finally {
      setLoading(false);
    }
  };

  // YAML format validation (frontend)
  const validateYaml = (content: string): boolean => {
    try {
      // Simple validation: check basic syntax
      // Empty content not allowed
      if (!content.trim()) {
        setYamlError('配置不能为空');
        return false;
      }
      // Check for obvious syntax errors (like unmatched quotes)
      // Here we use simple check, actual parsing is handled by backend
      setYamlError(null);
      return true;
    } catch {
      setYamlError('配置格式错误');
      return false;
    }
  };

  const handleSave = async () => {
    if (!validateYaml(yamlContent)) {
      return;
    }

    setSaving(true);
    try {
      const result = await configApi.saveConfig(yamlContent);
      if (result.success) {
        message.success('配置已保存');
        // Show restart prompt modal
        setRestartModalVisible(true);
      } else {
        message.error(result.error || '保存失败');
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleRestart = async () => {
    setRestartModalVisible(false);
    try {
      // 1. Stop service (with agent validation)
      const stopResult = await serviceApi.stop();
      if (!stopResult.success) {
        // Stop failed, show error
        Modal.warning({
          title: '无法重启服务',
          content: stopResult.error || '停止服务失败',
        });
        return;
      }

      // 2. Start service
      const startResult = await serviceApi.start();
      if (!startResult.success) {
        message.error(startResult.error || '启动服务失败');
        return;
      }

      message.success('服务已重启');
      onRestartRequired?.();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '重启服务失败');
    }
  };

  const handleChange = (value: string) => {
    setYamlContent(value);
    // Clear error prompt
    if (yamlError) {
      setYamlError(null);
    }
  };

  return (
    <>
      <Modal
        title="系统配置"
        open={open}
        onCancel={onCancel}
        width={700}
        footer={[
          <Button key="cancel" onClick={onCancel}>
            取消
          </Button>,
          <Button key="save" type="primary" loading={saving} onClick={handleSave}>
            保存
          </Button>,
        ]}
        destroyOnClose
      >
        {loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : yamlError ? (
          <Alert type="error" showIcon message={yamlError} />
        ) : (
          <div style={{ height: 500, overflow: 'auto' }}>
            <CodeMirror
              value={yamlContent}
              height="500px"
              extensions={[yaml()]}
              onChange={handleChange}
              theme="light"
              style={{
                fontSize: 13,
                fontFamily: 'Consolas, Monaco, monospace',
              }}
            />
          </div>
        )}
      </Modal>

      {/* Restart prompt modal */}
      <Modal
        title="配置已更新"
        open={restartModalVisible}
        onCancel={() => setRestartModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setRestartModalVisible(false)}>
            关闭
          </Button>,
          <Button key="restart" type="primary" onClick={handleRestart}>
            重启服务
          </Button>,
        ]}
      >
        <p>配置已更新，重启服务生效。</p>
      </Modal>
    </>
  );
};

export default ConfigEditorModal;