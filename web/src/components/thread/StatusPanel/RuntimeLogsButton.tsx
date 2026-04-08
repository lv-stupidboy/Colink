import React from 'react';
import { Button, Drawer, Typography } from 'antd';
import { FileTextOutlined } from '@ant-design/icons';

const { Paragraph } = Typography;

interface Props {
  invocationId?: string;
  logs?: string[];
}

export const RuntimeLogsButton: React.FC<Props> = ({ invocationId, logs = [] }) => {
  const [visible, setVisible] = React.useState(false);

  if (!invocationId) return null;

  return (
    <>
      <Button
        type="text"
        size="small"
        icon={<FileTextOutlined />}
        onClick={() => setVisible(true)}
        className="runtime-logs-btn"
      >
        日志
      </Button>
      <Drawer
        title={`运行日志 - ${invocationId.slice(0, 8)}`}
        placement="right"
        width={500}
        onClose={() => setVisible(false)}
        open={visible}
      >
        {logs.length === 0 ? (
          <Paragraph type="secondary">暂无日志</Paragraph>
        ) : (
          <div className="runtime-logs">
            {logs.map((log, index) => (
              <Paragraph key={index} className="log-line">
                <pre>{log}</pre>
              </Paragraph>
            ))}
          </div>
        )}
      </Drawer>
    </>
  );
};

export default RuntimeLogsButton;