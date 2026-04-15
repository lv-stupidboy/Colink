import React, { useState, memo, useCallback, useMemo } from 'react';
import { Typography, Tag, Button, Input, Space, Checkbox, Radio } from 'antd';
import { CheckCircleOutlined, SendOutlined } from '@ant-design/icons';
import type { QuestionBlock, ContentBlockStatus } from '@/types';
import './ContentBlock.css';

const { Text } = Typography;

/** 格式化执行时间 */
function formatDuration(ms?: number): string {
  if (!ms) return '';
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return rem > 0 ? `${m}m${rem}s` : `${m}m`;
}

/** 状态图标 */
function StatusIcon({ status, color }: { status: ContentBlockStatus; color?: string }) {
  if (status === 'streaming') {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke={color || '#1890ff'}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        style={{ animation: 'spin 1s linear infinite' }}
      >
        <path d="M21 12a9 9 0 1 1-6.219-8.56" />
      </svg>
    );
  }
  if (status === 'waiting_user_input') {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke={color || '#faad14'}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="10" />
        <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
        <line x1="12" y1="17" x2="12.01" y2="17" />
      </svg>
    );
  }
  if (status === 'success') {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke="#52c41a"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polyline points="20 6 9 17 4 12" />
      </svg>
    );
  }
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#ff4d4f"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="10" />
      <line x1="15" y1="9" x2="9" y2="15" />
      <line x1="9" y1="9" x2="15" y2="15" />
    </svg>
  );
}

/** Chevron 图标 */
function ChevronIcon({ expanded, color }: { expanded: boolean; color?: string }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke={color || '#8c8c8c'}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{
        transition: 'transform 0.15s',
        transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
      }}
    >
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

/** 扳手图标 */
function WrenchIcon({ color }: { color?: string }) {
  return (
    <svg
      width="11"
      height="11"
      viewBox="0 0 24 24"
      fill="none"
      stroke={color || '#8c8c8c'}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    </svg>
  );
}

interface QuestionBlockComponentProps {
  block: QuestionBlock;
  onSubmit?: (answers: Record<number, string | string[]>) => void;
  defaultExpanded?: boolean;
  disabled?: boolean; // 当 agent 未完成时，按钮禁用
}

/**
 * AskUserQuestion 内联显示组件
 * 将问题选项直接展示在对话中，而非弹框
 *
 * 交互规则：
 * - 单选：点击选项直接提交（选择即回答）
 * - 多选：选择后需要点击确认按钮提交
 * - 用户自定义输入：显示输入框 + 提交按钮
 * - disabled：当 agent 未完成时，按钮禁用，防止过快点击
 * - 已提交后：选项仍显示但禁用，选中项高亮
 * - Agent执行中：显示提示"待Agent执行完毕后可以选择"
 */
const QuestionBlockComponent: React.FC<QuestionBlockComponentProps> = memo(({ block, onSubmit, defaultExpanded = false, disabled = false }) => {
  const { toolName, questions, status, startedAt, completedAt, output, input } = block;
  const accentColor = '#faad14';

  // 调试日志：打印 block 的完整数据
  console.log('[QuestionBlock] Block data:', {
    id: block.id,
    toolName,
    status,
    output,
    questionsCount: questions?.length,
    questions: questions,
    input: input,
    inputQuestions: input?.questions,
    startedAt,
    completedAt,
    disabled,
  });

  // 防御性检查：确保 questions 存在
  // 如果 questions 字段为空，尝试从 input.questions 中提取（兼容历史数据）
  const safeQuestions = questions || (input?.questions as any[]) || [];

  // 是否已提交（status 为 success 或 failed）
  const isSubmitted = status === 'success' || status === 'failed';

  // 提交中状态：用于在提交后等待后端响应期间禁用交互
  const [isSubmitting, setIsSubmitting] = useState(false);

  // 是否等待 Agent 执行完毕（disabled=true 且 status=waiting_user_input）
  const isWaitingForAgent = disabled && status === 'waiting_user_input';

  // 是否禁用交互（agent 未完成、已提交、或正在提交中）
  const isInteractionDisabled = disabled || isSubmitted || isSubmitting;

  // 从 output 解析用户答案（用于高亮已提交的选项）
  // output 格式：单选为单个字符串，多选为用分隔符连接的多个字符串
  const parseOutputAnswers = (questionIndex: number, multiSelect: boolean): string | string[] | undefined => {
    if (!output) return undefined;
    const question = safeQuestions[questionIndex];
    if (!question) return undefined;

    if (multiSelect) {
      // 多选：output 可能包含多个答案
      const outputParts = output.split(/[、,\n]/).filter(p => p.trim());
      // 找出哪些选项被选中
      const selectedLabels: string[] = [];
      for (const option of question.options) {
        if (outputParts.includes(option.label)) {
          selectedLabels.push(option.label);
        }
      }
      // 也检查自定义输入（不在选项列表中的）
      const customAnswers = outputParts.filter(p => !question.options.some(o => o.label === p));
      if (customAnswers.length > 0) {
        selectedLabels.push(...customAnswers);
      }
      return selectedLabels.length > 0 ? selectedLabels : undefined;
    } else {
      // 单选：找匹配的选项或自定义答案
      const trimmedOutput = output.trim();
      for (const option of question.options) {
        if (option.label === trimmedOutput) {
          return option.label;
        }
      }
      // 自定义答案
      return trimmedOutput;
    }
  };

  // 存储每个问题的答案（单选为 string，多选为 string[]）
  // 已提交时使用解析的 output，未提交时使用本地 state
  const [localAnswers, setLocalAnswers] = useState<Record<number, string | string[]>>({});

  // 获取 answers：优先从 output 解析，其次从 input 解析（兼容不同数据格式）
  const answers = useMemo(() => {
    console.log('[QuestionBlock] Computing answers:', { isSubmitted, output, safeQuestionsLength: safeQuestions.length, status, localAnswers });

    if (isSubmitted) {
      // 已提交：尝试从 output 解析
      const parsedAnswers = safeQuestions.reduce((acc, q, i) => {
        const parsed = parseOutputAnswers(i, q.multiSelect);
        console.log('[QuestionBlock] Parsed answer:', { questionIndex: i, parsed, output });
        if (parsed) acc[i] = parsed;
        return acc;
      }, {} as Record<number, string | string[]>);

      console.log('[QuestionBlock] Parsed answers:', parsedAnswers);

      // 如果 output 解析成功，返回解析结果
      if (Object.keys(parsedAnswers).length > 0) {
        return parsedAnswers;
      }

      // 如果 output 解析失败，尝试从 input 中获取（可能存在 annotations 等字段）
      // 这是为了兼容不同的数据格式
      const inputAnswers = safeQuestions.reduce((acc, q, i) => {
        // 检查是否有用户答案的记录（兼容扩展字段）
        const questionAny = q as any;
        if (questionAny.userAnswer) {
          acc[i] = questionAny.userAnswer;
        }
        return acc;
      }, {} as Record<number, string | string[]>);

      if (Object.keys(inputAnswers).length > 0) {
        return inputAnswers;
      }
    }
    return localAnswers;
  }, [isSubmitted, safeQuestions, output, localAnswers]);

  // 存储用户自定义输入的值
  const [customInputs, setCustomInputs] = useState<Record<number, string>>({});
  // 展开状态
  const [expanded, setExpanded] = useState(status === 'waiting_user_input' || defaultExpanded);

  // 计算耗时
  const duration = completedAt ? completedAt - startedAt : 0;

  // 检查问题是否已回答
  const isQuestionAnswered = (index: number, multiSelect: boolean) => {
    const answer = answers[index];
    if (multiSelect) {
      return Array.isArray(answer) && answer.length > 0;
    }
    return typeof answer === 'string' && answer.length > 0;
  };

  // 检查所有问题是否已回答
  const allQuestionsAnswered = safeQuestions.every((q, i) => isQuestionAnswered(i, q.multiSelect));

  // 处理单选点击（选择即回答）
  const handleSingleSelect = useCallback((questionIndex: number, value: string) => {
    console.log('[QuestionBlock] handleSingleSelect:', { questionIndex, value, isInteractionDisabled, hasOnSubmit: !!onSubmit });
    if (isInteractionDisabled) return;
    if (!onSubmit) {
      console.log('[QuestionBlock] No onSubmit callback');
      return;
    }

    // 先更新 localAnswers 以立即显示选中状态（在提交过程中也能高亮）
    setLocalAnswers(prev => ({ ...prev, [questionIndex]: value }));

    // 检查是否需要自定义输入的选项
    const needsCustomInput = value.toLowerCase().includes('other') ||
                              value.includes('其他') ||
                              value.includes('自定义');

    if (needsCustomInput) {
      // 需要自定义输入：保持选中状态，等待用户输入
      // localAnswers 已更新，显示选中高亮
    } else {
      // 直接提交，设置提交中状态以禁用后续点击
      console.log('[QuestionBlock] Calling onSubmit');
      setIsSubmitting(true);
      onSubmit({ [questionIndex]: value });
    }
  }, [isInteractionDisabled, onSubmit]);

  // 处理多选变化
  const handleMultiSelectChange = useCallback((questionIndex: number, values: string[]) => {
    if (isInteractionDisabled) return;
    setLocalAnswers(prev => ({ ...prev, [questionIndex]: values }));
  }, [isInteractionDisabled]);

  // 处理自定义输入
  const handleCustomInputChange = useCallback((questionIndex: number, value: string) => {
    setCustomInputs(prev => ({ ...prev, [questionIndex]: value }));
  }, []);

  // 处理自定义输入提交
  const handleCustomInputSubmit = useCallback((questionIndex: number) => {
    if (isInteractionDisabled || !onSubmit) return;

    const customValue = customInputs[questionIndex];
    if (!customValue?.trim()) return;

    // 提交自定义输入，设置提交中状态以禁用后续点击
    setIsSubmitting(true);
    onSubmit({ [questionIndex]: customValue.trim() });
  }, [customInputs, isInteractionDisabled, onSubmit]);

  // 处理多选确认提交
  const handleMultiSelectSubmit = useCallback(() => {
    if (isInteractionDisabled || !onSubmit || !allQuestionsAnswered) return;
    // 设置提交中状态以禁用后续点击
    setIsSubmitting(true);
    onSubmit(answers);
  }, [answers, allQuestionsAnswered, isInteractionDisabled, onSubmit]);

  // 检查选项是否需要自定义输入
  const optionNeedsCustomInput = (label: string) => {
    return label.toLowerCase().includes('other') ||
           label.includes('其他') ||
           label.includes('自定义');
  };

  return (
    <div className="question-block-wrapper" style={{ marginTop: 8 }}>
      {/* Header - 工具调用行 */}
      <button
        type="button"
        className={`tool-call-row ${status}`}
        onClick={() => setExpanded(v => !v)}
      >
        {/* 状态图标 */}
        <span className="tool-call-icon">
          <StatusIcon status={status} color={accentColor} />
        </span>

        {/* 扳手图标 */}
        <WrenchIcon color={status === 'waiting_user_input' ? accentColor : undefined} />

        {/* 工具名称 */}
        <span
          className="tool-call-name"
          style={{ color: status === 'waiting_user_input' ? accentColor : undefined }}
        >
          {toolName}
        </span>

        {/* 耗时 */}
        {duration > 0 && (
          <span className="tool-call-duration">
            {formatDuration(duration)}
          </span>
        )}

        {/* 展开指示器 */}
        <ChevronIcon expanded={expanded} />
      </button>

      {/* Body - 问题选项 */}
      {expanded && (
        <div className="question-block-body" style={{
          padding: '12px 16px',
          background: 'var(--bg-container)',
          borderRadius: '8px',
          marginTop: '4px',
          border: `1px solid var(--border-color)`,
        }}>
          {safeQuestions.map((question, questionIndex) => {
            const currentAnswer = answers[questionIndex];
            const hasCustomInput = typeof currentAnswer === 'string' && optionNeedsCustomInput(currentAnswer);
            const customInputValue = customInputs[questionIndex] || '';

            return (
              <div key={questionIndex} style={{ marginBottom: questionIndex < safeQuestions.length - 1 ? 16 : 0 }}>
                {/* 问题标题 */}
                <Space style={{ marginBottom: 8 }}>
                  <Tag color="blue">{question.header}</Tag>
                  {question.multiSelect && <Tag color="orange">可多选</Tag>}
                  {isSubmitted && <Tag color="green" icon={<CheckCircleOutlined />}>已回答</Tag>}
                </Space>

                {/* 问题内容 */}
                <Text style={{ fontSize: 14, fontWeight: 500, display: 'block', marginBottom: 12 }}>
                  {question.question}
                </Text>

                {/* 状态提示：等待 Agent 执行完毕 */}
                {isWaitingForAgent && (
                  <div style={{
                    marginBottom: 12,
                    padding: '8px 12px',
                    background: 'var(--bg-secondary)',
                    borderRadius: 4,
                    borderLeft: '3px solid var(--color-primary)',
                  }}>
                    <Text type="secondary" style={{ fontSize: 13 }}>
                      ⏳ 待 Agent 执行完毕后可以选择
                    </Text>
                  </div>
                )}

                {/* 选项区域 - 始终显示，已提交时禁用 */}
                <div style={{ marginTop: 8 }}>
                  {question.multiSelect ? (
                    // 多选：使用 Checkbox
                    <Checkbox.Group
                      value={(answers[questionIndex] as string[]) || []}
                      onChange={(checkedValues) => handleMultiSelectChange(questionIndex, checkedValues as string[])}
                      style={{ width: '100%' }}
                      disabled={isInteractionDisabled}
                    >
                      {question.options.map((option) => {
                        const isSelected = (answers[questionIndex] as string[])?.includes(option.label);
                        return (
                          <Checkbox
                            key={option.label}
                            value={option.label}
                            style={{
                              marginBottom: 8,
                              width: '100%',
                              // 高亮选中项（已提交时更明显）
                              background: isSelected ? (isSubmitted ? 'var(--color-primary-bg)' : 'var(--color-primary-bg-hover)') : 'transparent',
                              borderRadius: 4,
                              padding: '4px 8px',
                              marginLeft: '-8px',
                              // 已提交时添加边框
                              border: isSelected && isSubmitted ? '1px solid var(--color-primary)' : 'none',
                            }}
                          >
                            <div>
                              <Text strong style={{ color: isSelected ? 'var(--color-primary)' : undefined }}>
                                {option.label}
                              </Text>
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
                                  background: 'var(--bg-secondary)',
                                  borderRadius: 4,
                                }}
                              >
                                {option.preview}
                              </Text>
                            )}
                          </Checkbox>
                        );
                      })}
                    </Checkbox.Group>

                    // 检查是否有需要自定义输入的选项被选中（仅未提交时）
                  ) : (
                    // 单选：使用 Radio
                    <Radio.Group
                      value={answers[questionIndex] as string}
                      onChange={(e) => handleSingleSelect(questionIndex, e.target.value)}
                      style={{ width: '100%' }}
                      disabled={isInteractionDisabled}
                    >
                      {question.options.map((option) => {
                        const isSelected = answers[questionIndex] === option.label;
                        return (
                          <Radio
                            key={option.label}
                            value={option.label}
                            style={{
                              marginBottom: 8,
                              width: '100%',
                              // 高亮选中项（已提交时更明显）
                              background: isSelected ? (isSubmitted ? 'var(--color-primary-bg)' : 'var(--color-primary-bg-hover)') : 'transparent',
                              borderRadius: 4,
                              padding: '4px 8px',
                              marginLeft: '-8px',
                              // 已提交时添加边框
                              border: isSelected && isSubmitted ? '1px solid var(--color-primary)' : 'none',
                            }}
                          >
                            <div>
                              <Text strong style={{ color: isSelected ? 'var(--color-primary)' : undefined }}>
                                {option.label}
                              </Text>
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
                                  background: 'var(--bg-secondary)',
                                  borderRadius: 4,
                                }}
                              >
                                {option.preview}
                              </Text>
                            )}
                          </Radio>
                        );
                      })}
                    </Radio.Group>
                  )}

                  {/* 单选自定义输入场景：选中自定义选项后显示输入框（仅未提交时） */}
                  {!isSubmitted && !question.multiSelect && hasCustomInput && (
                    <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
                      <Input
                        placeholder="请输入自定义内容..."
                        value={customInputValue}
                        onChange={(e) => handleCustomInputChange(questionIndex, e.target.value)}
                        style={{ flex: 1 }}
                        autoFocus
                        disabled={disabled}
                      />
                      <Button
                        type="primary"
                        icon={<SendOutlined />}
                        onClick={() => handleCustomInputSubmit(questionIndex)}
                        disabled={disabled || !customInputValue.trim()}
                      >
                        提交
                      </Button>
                    </div>
                  )}

                  {/* 已提交时的自定义答案显示 */}
                  {isSubmitted && !question.multiSelect && answers[questionIndex] && !question.options.some(o => o.label === answers[questionIndex]) && (
                    <div style={{
                      marginTop: 8,
                      padding: '8px 12px',
                      background: 'var(--color-primary-bg)',
                      borderRadius: 4,
                      border: '1px solid var(--color-primary)',
                    }}>
                      <Text style={{ color: 'var(--color-primary)' }}>
                        <CheckCircleOutlined style={{ marginRight: 4 }} />
                        自定义答案：{answers[questionIndex] as string}
                      </Text>
                    </div>
                  )}
                </div>
              </div>
            );
          })}

          {/* 多选确认按钮（仅多选且有未提交时显示） */}
          {!isSubmitted && safeQuestions.some(q => q.multiSelect) && allQuestionsAnswered && (
            <div style={{ marginTop: 16, textAlign: 'right' }}>
              <Button
                type="primary"
                icon={<CheckCircleOutlined />}
                onClick={handleMultiSelectSubmit}
                disabled={isInteractionDisabled}
              >
                确认提交
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
});

QuestionBlockComponent.displayName = 'QuestionBlockComponent';

export default QuestionBlockComponent;