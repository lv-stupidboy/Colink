import React, { useState, useEffect } from 'react';
import { Progress, Button, Tag, Alert, message, Space, Typography } from 'antd';
import { CheckCircleOutlined, LoadingOutlined, CloseCircleOutlined, RightOutlined, WarningOutlined } from '@ant-design/icons';
import { installApi, modeApi } from '../../../lib/api';
import type { InstallProgress } from '../../../lib/api/types';

const { Text } = Typography;

interface InstallConfig {
  installDir: string;
  createShortcut: boolean;
  launchNow: boolean;
  keepData: boolean;
  dependencies: any[];
  database?: { type: string };
  serverPort?: number;
  webPort?: number;
  configYaml?: string;
}

interface InstallingProps {
  config: InstallConfig;
  installType?: 'fresh' | 'upgrade' | 'reinstall';
  oldInstallDir?: string;
  onComplete: () => void;
}

interface StepProgress {
  step: string;
  label: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'warning';
  progress: number;
  message?: string;
  details?: string;
  startTime?: number;
  endTime?: number;
}

// 升级安装步骤
const UPGRADE_STEPS = [
  { key: 'prepare', label: '准备升级', description: '检查磁盘空间，停止运行中的进程' },
  { key: 'copy', label: '复制新版本', description: '复制新版本程序文件到安装目录' },
  { key: 'launcher', label: '复制 Launcher', description: '复制启动器程序' },
  { key: 'dbcheck', label: '检测数据库变更', description: '检查是否需要执行数据库迁移' },
  { key: 'migration', label: '数据库迁移', description: '执行 SQLite 数据库迁移脚本' },
  { key: 'config', label: '生成配置文件', description: '合并用户配置与模板配置' },
  { key: 'shortcut', label: '创建快捷方式', description: '创建桌面和开始菜单快捷方式' },
  { key: 'registry', label: '写入注册表', description: '注册安装信息到系统' },
];

// 重新安装步骤
const REINSTALL_STEPS = [
  { key: 'prepare', label: '准备重新安装', description: '检查磁盘空间，停止运行中的进程' },
  { key: 'uninstall', label: '卸载旧版本', description: '删除旧目录程序文件和快捷方式' },
  { key: 'migratedata', label: '迁移数据', description: '将旧目录 data 迁移到新目录（如保留数据）' },
  { key: 'copy', label: '复制新版本', description: '复制新版本程序文件到安装目录' },
  { key: 'launcher', label: '复制 Launcher', description: '复制启动器程序' },
  { key: 'dbcheck', label: '检测数据库变更', description: '检查数据库迁移需求' },
  { key: 'migration', label: '数据库迁移', description: '执行数据库迁移（保留数据时执行差异迁移）' },
  { key: 'config', label: '生成配置文件', description: '创建配置文件（保留数据时合并配置）' },
  { key: 'shortcut', label: '创建快捷方式', description: '创建桌面和开始菜单快捷方式' },
  { key: 'registry', label: '写入注册表', description: '注册安装信息到系统' },
];

// 首次安装步骤
const INSTALL_STEPS = [
  { key: 'prepare', label: '准备安装', description: '检查磁盘空间' },
  { key: 'copy', label: '复制文件', description: '复制应用程序文件到安装目录' },
  { key: 'launcher', label: '复制 Launcher', description: '复制启动器程序' },
  { key: 'dbcheck', label: '检测数据库变更', description: '检查数据库初始化脚本' },
  { key: 'migration', label: '数据库初始化', description: '执行数据库初始化脚本' },
  { key: 'config', label: '生成配置文件', description: '创建默认配置文件' },
  { key: 'shortcut', label: '创建快捷方式', description: '创建桌面和开始菜单快捷方式' },
  { key: 'registry', label: '写入注册表', description: '注册安装信息到系统' },
];

// 合并所有步骤定义
const ALL_STEPS = [...UPGRADE_STEPS, ...REINSTALL_STEPS, ...INSTALL_STEPS.filter(s => !UPGRADE_STEPS.find(u => u.key === s.key) && !REINSTALL_STEPS.find(r => r.key === s.key))];

const Installing: React.FC<InstallingProps> = ({
  config,
  installType = 'fresh',
  oldInstallDir = '',
  onComplete,
}) => {
  // 根据安装类型选择步骤
  const getStepsByType = () => {
    switch (installType) {
      case 'upgrade': return UPGRADE_STEPS;
      case 'reinstall': return REINSTALL_STEPS;
      default: return INSTALL_STEPS;
    }
  };
  const currentSteps = getStepsByType();

  const [steps, setSteps] = useState<StepProgress[]>(
    currentSteps.map(s => ({
      step: s.key,
      label: s.label,
      status: 'pending',
      progress: 0
    }))
  );
  const [installError, setInstallError] = useState<string | null>(null);
  const [installComplete, setInstallComplete] = useState(false);
  const [expandedSteps, setExpandedSteps] = useState<string[]>([]);
  const [failedStep, setFailedStep] = useState<string | null>(null);
  const [isRetrying, setIsRetrying] = useState(false);
  const [isStarting, setIsStarting] = useState(true);
  const [eventListenerReady, setEventListenerReady] = useState(false);
  const [progressReceived, setProgressReceived] = useState<string[]>([]);
  const [version, setVersion] = useState<string>('1.0.0');

  // Get version on mount
  useEffect(() => {
    modeApi.getVersion().then(v => {
      console.log('[Installing] Got version:', v);
      setVersion(v);
    }).catch(e => {
      console.warn('[Installing] Failed to get version:', e);
      setVersion('1.0.0');
    });
  }, []);

  useEffect(() => {
    let unlisten: (() => void) | undefined;
    let isMounted = true;

    console.log('[Installing] useEffect running, config:', config);
    console.log('[Installing] installType:', installType);
    console.log('[Installing] oldInstallDir:', oldInstallDir);

    // 直接使用 listen API
    const setupListener = async () => {
      try {
        console.log('[Installing] Setting up event listener...');
        const { listen } = await import('@tauri-apps/api/event');

        unlisten = await listen<InstallProgress>('install-progress', (event) => {
          const progress = event.payload;

          if (!isMounted) {
            console.log('[Installing] Event received but component unmounted');
            return;
          }

          console.log('[Install Progress] Received:', progress);
          console.log('[Install Progress] step:', progress.step, 'status:', progress.status);

          setProgressReceived(prev => [...prev, `${progress.step}:${progress.status}`]);

          setSteps(prev => prev.map(s => {
            if (s.step === progress.step) {
              console.log('[Install Progress] Updating step:', s.step);
              return {
                ...s,
                status: progress.status as any,
                progress: progress.progress || 0,
                message: progress.message,
                details: progress.details,
                endTime: progress.status !== 'running' ? Date.now() : undefined,
                startTime: s.startTime || (progress.status === 'running' ? Date.now() : undefined)
              };
            }
            return s;
          }));

          if (progress.status === 'failed') {
            console.log('[Install Progress] Step failed:', progress.step, progress.message);
            setInstallError(progress.message || `${progress.step} 失败`);
            setFailedStep(progress.step);
            setIsStarting(false);
          }

          // registry 成功时完成
          if (progress.status === 'success' && progress.step === 'registry') {
            console.log('[Install Progress] Installation complete!');
            setInstallComplete(true);
            setIsStarting(false);
          }
        });

        console.log('[Installing] Event listener set up successfully');
        setEventListenerReady(true);

        // 事件监听设置好后，再启动安装
        if (!isRetrying && isMounted) {
          const installParams = {
            installDir: config.installDir,
            installMode: installType === 'upgrade' ? 'upgrade' : installType === 'reinstall' ? 'reinstall' : 'install',
            installType: installType, // Pass install type to backend
            oldInstallDir: installType === 'reinstall' ? oldInstallDir : undefined,
            keepData: config.keepData,
            database: config.database || { type: 'sqlite' },
            serverPort: config.serverPort || 26305,
            webPort: config.webPort || 26306,
            createShortcut: config.createShortcut,
            newVersion: version,
            configYaml: config.configYaml,
          };
          console.log('[Installing] Starting installation with params:', installParams);

          try {
            const result = await installApi.startInstallation(installParams);
            if (!isMounted) return;

            console.log('[Installing] Installation result:', result);
            setIsStarting(false);

            if (!result.success) {
              console.log('[Installing] Installation failed:', result.error);
              setInstallError(result.error || '安装失败');
              message.error(result.error || '安装失败');
            }
          } catch (installErr) {
            if (!isMounted) return;
            console.error('[Installing] Installation API error:', installErr);
            setIsStarting(false);
            const errorMsg = installErr instanceof Error ? installErr.message : String(installErr);
            setInstallError(errorMsg);
            message.error(errorMsg);
          }
        } else {
          console.log('[Installing] Skipping installation start:', { isRetrying, isMounted });
        }
      } catch (err) {
        console.error('[Installing] Failed to set up event listener:', err);
        const errorMsg = err instanceof Error ? err.message : String(err);
        setInstallError(`无法设置事件监听器: ${errorMsg}`);
        setIsStarting(false);
      }
    };

    setupListener();

    return () => {
      console.log('[Installing] useEffect cleanup');
      isMounted = false;
      unlisten?.();
    };
  }, [config, isRetrying, installType, oldInstallDir, version]);

  // 监控状态变化
  useEffect(() => {
    console.log('[Installing] State changed:', {
      isStarting,
      eventListenerReady,
      progressReceived: progressReceived.length,
      installError,
      installComplete
    });
  }, [isStarting, eventListenerReady, progressReceived, installError, installComplete]);

  // 重试安装
  const handleRetry = async () => {
    console.log('Retrying installation...');
    setIsRetrying(true);
    setInstallError(null);
    setFailedStep(null);
    setInstallComplete(false);
    setIsStarting(true);
    setEventListenerReady(false);
    setProgressReceived([]);

    // 重置步骤状态
    setSteps(prev => prev.map(s => {
      if (s.status === 'failed' || s.status === 'pending') {
        return { ...s, status: 'pending', progress: 0, message: undefined, details: undefined };
      }
      return s;
    }));

    // 重新启动安装
    const installParams = {
      installDir: config.installDir,
      installMode: installType === 'upgrade' ? 'upgrade' : installType === 'reinstall' ? 'reinstall' : 'install',
      installType: installType,
      oldInstallDir: installType === 'reinstall' ? oldInstallDir : undefined,
      keepData: config.keepData,
      database: config.database || { type: 'sqlite' },
      serverPort: config.serverPort || 26305,
      webPort: config.webPort || 26306,
      createShortcut: config.createShortcut,
      newVersion: version,
      configYaml: config.configYaml,
    };
    installApi.startInstallation(installParams).then(result => {
      if (!result.success) {
        setInstallError(result.error || '安装失败');
        message.error(result.error || '安装失败');
      }
      setIsRetrying(false);
      setIsStarting(false);
    }).catch(err => {
      console.error('Retry error:', err);
      setInstallError(err instanceof Error ? err.message : '安装过程出错');
      setIsRetrying(false);
      setIsStarting(false);
    });
  };

  const getStepIcon = (status: string) => {
    switch (status) {
      case 'success': return <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 18 }} />;
      case 'running': return <LoadingOutlined style={{ color: '#10b981', fontSize: 18 }} spin />;
      case 'failed': return <CloseCircleOutlined style={{ color: '#ff4d4f', fontSize: 18 }} />;
      case 'warning': return <WarningOutlined style={{ color: '#faad14', fontSize: 18 }} />;
      default: return <span style={{ color: '#d9d9d9', fontSize: 18 }}>○</span>;
    }
  };

  const getStepTag = (status: string) => {
    switch (status) {
      case 'success': return <Tag color="success">完成</Tag>;
      case 'running': return <Tag color="processing">进行中</Tag>;
      case 'failed': return <Tag color="error">失败</Tag>;
      case 'warning': return <Tag color="warning">注意</Tag>;
      default: return <Tag color="default">等待中</Tag>;
    }
  };

  const toggleStepExpand = (stepKey: string) => {
    setExpandedSteps(prev =>
      prev.includes(stepKey)
        ? prev.filter(s => s !== stepKey)
        : [...prev, stepKey]
    );
  };

  // 计算总体进度
  const completedSteps = steps.filter(s => s.status === 'success' || s.status === 'warning').length;
  const totalProgress = Math.round((completedSteps / steps.length) * 100);

  // 获取标题文本
  const getTitleText = () => {
    if (installError) {
      return installType === 'upgrade' ? '升级失败' : installType === 'reinstall' ? '重新安装失败' : '安装失败';
    }
    if (installComplete) {
      return installType === 'upgrade' ? '升级完成' : installType === 'reinstall' ? '重新安装完成' : '安装完成';
    }
    return installType === 'upgrade' ? '正在升级...' : installType === 'reinstall' ? '正在重新安装...' : '正在安装...';
  };

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>{getTitleText()}</h2>
      <p style={{ color: '#666', marginBottom: 20 }}>
        {installError ? '安装过程中遇到错误，请检查后重试' :
         installComplete ? '所有步骤已完成，请点击完成按钮继续' :
         '请稍候，安装程序正在配置您的系统'}
      </p>

      {/* 调试信息 - 仅在安装过程中显示 */}
      {isStarting && !installError && !installComplete && (
        <div style={{ marginBottom: 16, padding: 12, background: '#f5f5f5', borderRadius: 8, fontSize: 12 }}>
          <div style={{ marginBottom: 4 }}>事件监听器: {eventListenerReady ? '✓ 已就绪' : '⏳ 设置中...'}</div>
          <div style={{ marginBottom: 4 }}>安装启动: {isStarting ? '⏳ 正在启动...' : '✓ 已启动'}</div>
          <div style={{ marginBottom: 4 }}>收到进度事件: {progressReceived.length} 个</div>
          {progressReceived.length > 0 && (
            <div style={{ marginTop: 8, color: '#666' }}>
              事件列表: {progressReceived.slice(-5).join(', ')}
            </div>
          )}
        </div>
      )}

      {/* 总体进度条 */}
      {!installError && (
        <div style={{ marginBottom: 24 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <span>总体进度</span>
            <span>{completedSteps} / {steps.length} 步骤</span>
          </div>
          <Progress
            percent={totalProgress}
            status={installComplete ? 'success' : 'active'}
            strokeColor={{
              '0%': '#10b981',
              '100%': '#059669',
            }}
          />
        </div>
      )}

      {/* 安装成功提示 */}
      {installComplete && !installError && (
        <Alert
          type="success"
          showIcon
          style={{ marginBottom: 20 }}
          message={installType === 'upgrade' ? '升级成功' : installType === 'reinstall' ? '重新安装成功' : '安装成功'}
          description={
            <div>
              <p style={{ marginBottom: 8 }}>安装目录：{config.installDir}</p>
              <p style={{ marginBottom: 0 }}>请点击"完成"关闭安装程序，然后通过桌面快捷方式启动 Colink。</p>
            </div>
          }
        />
      )}

      {/* 安装错误提示 */}
      {installError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 20 }}
          message="安装失败"
          description={
            <div>
              <p style={{ marginBottom: 8 }}>{installError}</p>
              {failedStep && (
                <div style={{ marginBottom: 8, color: '#666' }}>
                  <strong>处理建议：</strong>
                  {failedStep === 'prepare' && '请检查磁盘空间是否充足'}
                  {failedStep === 'uninstall' && '请检查是否有程序占用目标文件'}
                  {failedStep === 'migratedata' && '请检查磁盘空间是否充足'}
                  {failedStep === 'copy' && '请检查磁盘空间是否充足，或是否有程序占用目标文件'}
                  {failedStep === 'migration' && '请检查数据库文件是否正常，可尝试手动执行数据库迁移脚本'}
                  {failedStep === 'config' && '请检查安装目录权限，确保可写入配置文件'}
                  {failedStep === 'shortcut' && '请检查桌面和开始菜单目录权限'}
                  {failedStep === 'registry' && '请检查注册表写入权限，可能需要管理员权限运行'}
                </div>
              )}
              <p style={{ marginBottom: 0, color: '#999' }}>处理完成后，点击"重试"按钮继续安装</p>
            </div>
          }
        />
      )}

      {/* 步骤列表 */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {steps.map((step) => {
          const stepDef = ALL_STEPS.find(s => s.key === step.step);
          const isExpanded = expandedSteps.includes(step.step);
          const hasDetails = step.details || step.message || step.status === 'failed';

          return (
            <div
              key={step.step}
              style={{
                border: '1px solid #f0f0f0',
                borderRadius: 8,
                marginBottom: 8,
                overflow: 'hidden',
                background: step.status === 'running' ? '#f6ffed' : '#fff',
              }}
            >
              {/* 步骤头部 */}
              <div
                onClick={() => hasDetails && toggleStepExpand(step.step)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                  padding: '12px 16px',
                  cursor: hasDetails ? 'pointer' : 'default',
                  transition: 'background 0.2s',
                }}
              >
                <div style={{ width: 24 }}>{getStepIcon(step.status)}</div>
                <div style={{ flex: 1 }}>
                  <div style={{ fontWeight: 500 }}>{step.label}</div>
                  {step.status === 'running' && step.message && (
                    <div style={{ fontSize: 12, color: '#666', marginTop: 2 }}>{step.message}</div>
                  )}
                </div>
                {step.status === 'running' && (
                  <div style={{ width: 120 }}>
                    <Progress percent={step.progress} size="small" />
                  </div>
                )}
                {getStepTag(step.status)}
                {hasDetails && (
                  <RightOutlined
                    style={{
                      fontSize: 12,
                      color: '#999',
                      transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
                      transition: 'transform 0.2s'
                    }}
                  />
                )}
              </div>

              {/* 展开的详情 */}
              {isExpanded && hasDetails && (
                <div style={{
                  padding: '12px 16px',
                  background: '#fafafa',
                  borderTop: '1px solid #f0f0f0',
                }}>
                  {stepDef?.description && (
                    <div style={{ marginBottom: 8, color: '#666', fontSize: 13 }}>
                      {stepDef.description}
                    </div>
                  )}
                  {step.details && (
                    <pre style={{
                      margin: 0,
                      padding: 8,
                      background: '#fff',
                      borderRadius: 4,
                      fontSize: 12,
                      fontFamily: 'Consolas, Monaco, monospace',
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-word',
                      border: '1px solid #e8e8e8',
                    }}>
                      {step.details}
                    </pre>
                  )}
                  {step.status === 'failed' && step.message && (
                    <div style={{ color: '#ff4d4f', fontSize: 13, marginTop: 8 }}>
                      错误信息：{step.message}
                    </div>
                  )}
                  {step.endTime && step.startTime && (
                    <div style={{ color: '#999', fontSize: 12, marginTop: 8 }}>
                      耗时：{((step.endTime - step.startTime) / 1000).toFixed(1)}s
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* 底部按钮 */}
      <div style={{ marginTop: 24, textAlign: 'right' }}>
        {installError ? (
          <Space>
            <Button onClick={() => window.close()}>
              关闭
            </Button>
            <Button type="primary" onClick={handleRetry} loading={isRetrying}>
              重试
            </Button>
          </Space>
        ) : installComplete ? (
          <Button type="primary" size="large" onClick={onComplete}>
            完成
          </Button>
        ) : null}
      </div>
    </div>
  );
};

export default Installing;