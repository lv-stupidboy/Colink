import React, { useEffect, useState } from 'react';
import { Button, Space, Spin, Typography } from 'antd';
import { modeApi, installApi, windowApi } from './lib/api';
import type { InstalledVersion } from './lib/api/types';

// Pages
import Welcome from './renderer/src/pages/Welcome';
import DirectorySelect from './renderer/src/pages/DirectorySelect';
import DependencyCheck from './renderer/src/pages/DependencyCheck';
import SystemConfig from './renderer/src/pages/SystemConfig';
import Installing from './renderer/src/pages/Installing';
import Complete from './renderer/src/pages/Complete';
import SelectAction from './renderer/src/pages/SelectAction';
import LauncherDashboard from './renderer/src/pages/LauncherDashboard';
import Uninstall from './renderer/src/pages/Uninstall';

// Components
import Layout from './renderer/src/components/Layout';

const { Text, Title } = Typography;

// 安装配置
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

// 步骤页面映射
const INSTALL_PAGES = {
  1: Welcome,
  2: DirectorySelect,
  3: DependencyCheck,
  4: SystemConfig,
};

const STEP_LABELS = ['欢迎', '目录选择', '智能体检测', '系统配置'];
const UPGRADE_STEP_LABELS = ['欢迎', '智能体检测', '系统配置'];

type AppMode = 'checking' | 'launcher' | 'select-action' | 'install' | 'installing' | 'complete' | 'uninstall';

type InstallType = 'fresh' | 'upgrade' | 'reinstall';

function App() {
  const [mode, setMode] = useState<AppMode>('checking');
  const [currentStep, setCurrentStep] = useState(1);
  const [installedVersion, setInstalledVersion] = useState<InstalledVersion | null>(null);
  const [installType, setInstallType] = useState<InstallType>('fresh');
  const [config, setConfig] = useState<InstallConfig>({
    installDir: 'C:\\Program Files\\Colink',
    createShortcut: true,
    launchNow: true,
    keepData: true,
    dependencies: [],
  });
  const [dirValid, setDirValid] = useState(true);
  const [serviceStatus, setServiceStatus] = useState<'running' | 'stopped'>('stopped');

  // 升级模式跳过目录选择
  const isUpgrade = installType === 'upgrade';
  const isReinstall = installType === 'reinstall';
  const getStepLabels = () => isUpgrade ? UPGRADE_STEP_LABELS : STEP_LABELS;

  // 旧安装目录（用于重新安装时迁移数据）
  const oldInstallDir = installedVersion?.installDir || '';

  useEffect(() => {
    checkInstalledVersion();
  }, []);

  // 定期更新服务状态
  useEffect(() => {
    if (mode === 'launcher') {
      updateServiceStatus();
      const timer = setInterval(updateServiceStatus, 5000);
      return () => clearInterval(timer);
    }
  }, [mode]);

  const checkInstalledVersion = async () => {
    try {
      // 首先检测是否是 Launcher 模式
      const isLauncher = await modeApi.isLauncherMode();
      if (isLauncher) {
        const result = await installApi.checkInstalled();
        setInstalledVersion(result);
        setMode('launcher');
        return;
      }

      // Setup 模式
      const result = await installApi.checkInstalled();
      setInstalledVersion(result);

      if (result.installed && result.installDir) {
        setConfig(prev => ({ ...prev, installDir: result.installDir!, keepData: true }));
        setMode('select-action');
        return;
      }

      // 全新安装
      setMode('install');
    } catch {
      setMode('install');
    }
  };

  const updateServiceStatus = async () => {
    try {
      // TODO: 实现服务状态检查
      setServiceStatus('stopped');
    } catch {
      setServiceStatus('stopped');
    }
  };

  const handleNext = () => {
    if (currentStep >= 4) return;
    let nextStep = currentStep + 1;
    if (isUpgrade) {
      // 升级跳过目录选择(步骤2)：1->3->4
      if (currentStep === 1) nextStep = 3;
      else if (currentStep === 3) nextStep = 4;
    }
    setCurrentStep(nextStep);
  };

  const handlePrev = () => {
    if (currentStep <= 1) return;
    let prevStep = currentStep - 1;
    if (isUpgrade) {
      // 升级跳过目录选择(步骤2)：4->3->1
      if (currentStep === 4) prevStep = 3;
      else if (currentStep === 3) prevStep = 1;
    }
    setCurrentStep(prevStep);
  };

  const handleInstall = async () => {
    setMode('installing');
  };

  const handleStartUpgrade = () => {
    // 升级：保持原目录，跳过目录选择，保留数据
    setInstallType('upgrade');
    setConfig(prev => ({ ...prev, installDir: installedVersion?.installDir || prev.installDir, keepData: true }));
    setMode('install');
    setCurrentStep(1);
  };

  const handleStartReinstall = () => {
    // 重新安装：显示目录选择页面，预填之前路径，默认保留数据
    setInstallType('reinstall');
    setConfig(prev => ({ ...prev, installDir: installedVersion?.installDir || prev.installDir, keepData: true }));
    setMode('install');
    setCurrentStep(1);
  };

  const handleStartUninstall = () => {
    setMode('uninstall');
  };

  const handleUninstallComplete = () => {
    windowApi.close();
  };

  // 检测中
  if (mode === 'checking') {
    return (
      <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" tip="检测安装环境..." />
      </div>
    );
  }

  // Launcher 模式
  if (mode === 'launcher') {
    return (
      <Layout hideSteps title="Colink">
        <LauncherDashboard />
      </Layout>
    );
  }

  // 选择操作页面
  if (mode === 'select-action') {
    return (
      <Layout hideSteps title="Colink Setup">
        <SelectAction
          installDir={installedVersion?.installDir || ''}
          version={installedVersion?.version}
          onUpgrade={handleStartUpgrade}
          onReinstall={handleStartReinstall}
          onUninstall={handleStartUninstall}
          onCancel={() => windowApi.close()}
        />
      </Layout>
    );
  }

  // 卸载页面
  if (mode === 'uninstall') {
    return (
      <Layout hideSteps title="Colink Setup">
        <Uninstall
          installDir={installedVersion?.installDir || ''}
          onComplete={handleUninstallComplete}
          onCancel={() => setMode('select-action')}
        />
      </Layout>
    );
  }

  // 安装中
  if (mode === 'installing') {
    return (
      <Layout hideSteps title="Colink Setup">
        <Installing
          config={config}
          installType={installType}
          oldInstallDir={oldInstallDir}
          onComplete={() => setMode('complete')}
        />
      </Layout>
    );
  }

  // 安装完成
  if (mode === 'complete') {
    return (
      <Layout hideSteps title="Colink Setup">
        <Complete
          config={config}
          installType={installType}
        />
      </Layout>
    );
  }

  // 安装向导
  const PageComponent = INSTALL_PAGES[currentStep as keyof typeof INSTALL_PAGES];
  // 升级跳过目录选择(步骤2)：计算步骤索引
  const stepIndex = isUpgrade
    ? (currentStep === 1 ? 0 : currentStep === 3 ? 1 : 2)
    : currentStep - 1;

  // 获取安装按钮文本
  const getInstallButtonText = () => {
    if (isUpgrade) return '升级';
    if (isReinstall) return '重新安装';
    return '安装';
  };

  return (
    <Layout
      currentStep={stepIndex + 1}
      stepLabels={getStepLabels()}
      title="Colink Setup"
      isLauncherMode={false}
    >
      {PageComponent && (
        <PageComponent
          config={config}
          onConfigUpdate={(updates: Partial<InstallConfig>) => setConfig(prev => ({ ...prev, ...updates }))}
          onValidationChange={(valid: boolean) => {
            if (currentStep === 2) setDirValid(valid);
          }}
          installedVersion={installedVersion ?? undefined}
          installType={installType}
        />
      )}

      <div className="footer-buttons">
        <Space>
          {currentStep > 1 && !(isUpgrade && currentStep === 3) && !(isUpgrade && currentStep === 2) && (
            <Button onClick={handlePrev}>上一步</Button>
          )}
          {currentStep === 1 && (
            <Button type="primary" onClick={handleNext}>
              {isUpgrade ? '开始升级' : isReinstall ? '开始重新安装' : '开始安装'}
            </Button>
          )}
          {/* 步骤2：目录选择 - 首次安装和重新安装才显示 */}
          {currentStep === 2 && dirValid && (
            <Button type="primary" onClick={handleNext}>下一步</Button>
          )}
          {/* 步骤3：智能体检测 */}
          {currentStep === 3 && (
            <Button type="primary" onClick={handleNext}>下一步</Button>
          )}
          {currentStep === 4 && (
            <Button type="primary" onClick={handleInstall}>
              {getInstallButtonText()}
            </Button>
          )}
        </Space>
      </div>
    </Layout>
  );
}

export default App;