import React, { useState } from 'react';
import { Modal, Input, Button, Upload, Space, message } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import type { HumanTask, SubmitHumanTaskRequest } from '@/types';
import { api } from '@/api/client';

interface TaskExecuteModalProps {
  task: HumanTask;
  visible: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const TaskExecuteModal: React.FC<TaskExecuteModalProps> = ({
  task,
  visible,
  onClose,
  onSuccess,
}) => {
  const [outputContent, setOutputContent] = useState('');
  const [outputFiles, setOutputFiles] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    if (!outputContent.trim()) {
      message.warning('请填写交付内容');
      return;
    }

    setLoading(true);
    try {
      const req: SubmitHumanTaskRequest = {
        outputContent,
        outputFiles,
      };
      const result = await api.humanTasks.submit(task.id, req);

      if (result.success) {
        message.success('提交成功');
        if (result.triggered && result.nextAgent) {
          message.info(`已触发下游 Agent: ${result.nextAgent.name}`);
        }
        onSuccess();
        onClose();
      }
    } catch (err: any) {
      message.error(err.message || '提交失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={`执行任务: ${task.roleName}`}
      open={visible}
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>取消</Button>,
        <Button key="submit" type="primary" loading={loading} onClick={handleSubmit}>
          提交
        </Button>,
      ]}
      width={600}
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div>
          <div style={{ marginBottom: 8 }}>任务描述:</div>
          <div style={{ background: 'var(--bg-container)', padding: 12, borderRadius: 4 }}>
            {task.taskContent}
          </div>
        </div>

        <div>
          <div style={{ marginBottom: 8 }}>交付内容:</div>
          <Input.TextArea
            rows={6}
            value={outputContent}
            onChange={(e) => setOutputContent(e.target.value)}
            placeholder="请填写交付物内容..."
          />
        </div>

        <div>
          <div style={{ marginBottom: 8 }}>上传文件:</div>
          <Upload
            beforeUpload={(file) => {
              // 暂时只记录文件名，后续需要实现真实上传
              setOutputFiles([...outputFiles, file.name]);
              return false; // 阻止自动上传
            }}
            fileList={outputFiles.map((name, idx) => ({
              uid: `${idx}`,
              name,
              status: 'done',
            }))}
            onRemove={(file) => {
              setOutputFiles(outputFiles.filter((f) => f !== file.name));
            }}
          >
            <Button icon={<UploadOutlined />}>选择文件</Button>
          </Upload>
        </div>
      </Space>
    </Modal>
  );
};

export default TaskExecuteModal;