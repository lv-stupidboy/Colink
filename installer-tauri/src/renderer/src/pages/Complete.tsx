import React from 'react';
import { Button, Typography, Result } from 'antd';
import { windowApi } from '../../../lib/api';

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
        subTitle="Colink 已成功安装到您的计算机"
        extra={[
          <Button type="primary" key="close" onClick={handleClose}>
            关闭安装程序
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