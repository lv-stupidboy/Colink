import React from 'react';
import { Button, Typography, Result, message } from 'antd';
import { windowApi, launcherApi } from '../../../lib/api';

const { Text } = Typography;

interface InstallConfig {
  installDir: string;
  createShortcut: boolean;
  launchNow: boolean;
  keepData: boolean;
  dependencies: any[];
}

interface CompleteProps {
  config: InstallConfig;
  installType?: 'fresh' | 'upgrade' | 'reinstall';
}

const Complete: React.FC<CompleteProps> = ({
  config,
  installType,
}) => {
  const handleOpenInstallDir = async () => {
    try {
      await launcherApi.openInstallDir(config.installDir);
      message.success('已打开安装目录，请双击 Colink.exe 启动应用');
      // 延迟关闭 Setup，让用户看到提示
      setTimeout(() => {
        windowApi.close();
      }, 1000);
    } catch (err) {
      console.error('Failed to open install dir:', err);
      message.error(`无法打开安装目录: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  const handleClose = async () => {
    await windowApi.close();
  };

  const getTitle = () => {
    switch (installType) {
      case 'upgrade': return '升级完成';
      case 'reinstall': return '重新安装完成';
      default: return '安装完成';
    }
  };

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', textAlign: 'center' }}>
      <Result
        status="success"
        title={getTitle()}
        subTitle="Colink 已成功安装到您的计算机。点击下方按钮打开安装目录，然后双击 Colink.exe 启动应用。"
        extra={[
          <Button type="primary" key="open" onClick={handleOpenInstallDir}>
            打开安装目录并关闭安装程序
          </Button>,
          <Button key="close" onClick={handleClose}>
            仅关闭安装程序
          </Button>,
        ]}
      />

      <Text type="secondary" style={{ fontSize: 13 }}>
        安装位置: {config.installDir}
      </Text>
    </div>
  );
};

export default Complete;