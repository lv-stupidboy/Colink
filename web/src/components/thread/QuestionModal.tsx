import React, { useState } from 'react';
import { Modal, Typography, List, Radio, Checkbox, Button, Space, Tag } from 'antd';
import { QuestionCircleOutlined, CheckCircleOutlined } from '@ant-design/icons';
import type { QuestionItem } from '@/types';

interface QuestionModalProps {
  visible: boolean;
  questions: QuestionItem[];
  onSubmit: (answers: Record<number, string | string[]>) => void;
  onCancel?: () => void;
}

const { Text } = Typography;

/**
 * AskUserQuestion 弹窗组件
 * 显示 Agent 提出的问题，并捕获用户的答案
 */
const QuestionModal: React.FC<QuestionModalProps> = ({
  visible,
  questions,
  onSubmit,
  onCancel,
}) => {
  // 存储每个问题的答案（单选为 string，多选为 string[]）
  const [answers, setAnswers] = useState<Record<number, string | string[]>>({});

  const handleSubmit = () => {
    onSubmit(answers);
    // 重置答案
    setAnswers({});
  };

  const isQuestionAnswered = (index: number, multiSelect: boolean) => {
    const answer = answers[index];
    if (multiSelect) {
      return Array.isArray(answer) && answer.length > 0;
    }
    return typeof answer === 'string' && answer.length > 0;
  };

  const allQuestionsAnswered = questions.every((q, i) => isQuestionAnswered(i, q.multiSelect));

  return (
    <Modal
      open={visible}
      title={
        <Space>
          <QuestionCircleOutlined style={{ color: '#1890ff' }} />
          <span>Agent 需要您的输入</span>
        </Space>
      }
      onCancel={onCancel}
      footer={[
        <Button key="cancel" onClick={onCancel}>
          取消
        </Button>,
        <Button
          key="submit"
          type="primary"
          onClick={handleSubmit}
          disabled={!allQuestionsAnswered}
          icon={<CheckCircleOutlined />}
        >
          提交答案
        </Button>,
      ]}
      width={600}
      centered
    >
      <List
        dataSource={questions}
        renderItem={(question, questionIndex) => (
          <List.Item style={{ borderBottom: '1px solid #f0f0f0', paddingBottom: 16 }}>
            <div style={{ width: '100%' }}>
              {/* 问题标题 */}
              <Space style={{ marginBottom: 8 }}>
                <Tag color="blue">{question.header}</Tag>
                {question.multiSelect && <Tag color="orange">可多选</Tag>}
                {isQuestionAnswered(questionIndex, question.multiSelect) && (
                  <Tag color="green" icon={<CheckCircleOutlined />}>已回答</Tag>
                )}
              </Space>

              {/* 问题内容 */}
              <Text style={{ fontSize: 14, fontWeight: 500, display: 'block', marginBottom: 12 }}>
                {question.question}
              </Text>

              {/* 选项列表 */}
              <div style={{ marginTop: 8 }}>
                {question.multiSelect ? (
                  // 多选：使用 Checkbox
                  <Checkbox.Group
                    value={(answers[questionIndex] as string[] || [])}
                    onChange={(checkedValues) => {
                      setAnswers(prev => ({ ...prev, [questionIndex]: checkedValues as string[] }));
                    }}
                    style={{ width: '100%' }}
                  >
                    {question.options.map((option) => (
                      <Checkbox
                        key={option.label}
                        value={option.label}
                        style={{ marginBottom: 8, width: '100%' }}
                      >
                        <div>
                          <Text strong>{option.label}</Text>
                          {option.description && (
                            <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
                              {option.description}
                            </Text>
                          )}
                        </div>
                        {option.preview && (
                          <Text
                            type="secondary"
                            style={{
                              fontSize: 12,
                              display: 'block',
                              marginTop: 4,
                              padding: '4px 8px',
                              background: '#f5f5f5',
                              borderRadius: 4,
                            }}
                          >
                            {option.preview}
                          </Text>
                        )}
                      </Checkbox>
                    ))}
                  </Checkbox.Group>
                ) : (
                  // 单选：使用 Radio
                  <Radio.Group
                    value={answers[questionIndex] as string}
                    onChange={(e) => {
                      setAnswers(prev => ({ ...prev, [questionIndex]: e.target.value }));
                    }}
                    style={{ width: '100%' }}
                  >
                    {question.options.map((option) => (
                      <Radio
                        key={option.label}
                        value={option.label}
                        style={{ marginBottom: 8, width: '100%' }}
                      >
                        <div>
                          <Text strong>{option.label}</Text>
                          {option.description && (
                            <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
                              {option.description}
                            </Text>
                          )}
                        </div>
                        {option.preview && (
                          <Text
                            type="secondary"
                            style={{
                              fontSize: 12,
                              display: 'block',
                              marginTop: 4,
                              padding: '4px 8px',
                              background: '#f5f5f5',
                              borderRadius: 4,
                            }}
                          >
                            {option.preview}
                          </Text>
                        )}
                      </Radio>
                    ))}
                  </Radio.Group>
                )}
              </div>
            </div>
          </List.Item>
        )}
      />
    </Modal>
  );
};

export default QuestionModal;