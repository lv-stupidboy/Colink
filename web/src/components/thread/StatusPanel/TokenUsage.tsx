import React, { useState, useEffect, useRef } from 'react';
import { Progress } from 'antd';
import { RightOutlined } from '@ant-design/icons';
import type { TokenUsage as TokenUsageType } from '@/types/status';

interface Props {
  usage: Record<string, TokenUsageType>;
  totalUsage?: {
    input: number;
    output: number;
    cache: number;
    cost: number;
    contextUsed?: number;
    contextSize?: number;
  };
  defaultCollapsed?: boolean;
}

const formatTokens = (n: number): string => {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n);
};

const AnimatedValue: React.FC<{ value: string; className?: string }> = ({ value, className }) => {
  const [animate, setAnimate] = useState(false);
  const prevValue = useRef(value);

  useEffect(() => {
    if (prevValue.current !== value) {
      setAnimate(true);
      prevValue.current = value;
      const timer = setTimeout(() => setAnimate(false), 500);
      return () => clearTimeout(timer);
    }
  }, [value]);

  return (
    <span className={`${className || ''} ${animate ? 'token-updated' : ''}`}>
      {value}
    </span>
  );
};

export const TokenUsage: React.FC<Props> = ({ usage, totalUsage, defaultCollapsed = true }) => {
  const [collapsed, setCollapsed] = useState(defaultCollapsed);

  // 使用传入的 totalUsage 或自行计算
  const total = totalUsage || Object.values(usage).reduce(
    (acc, u) => ({
      input: acc.input + (u.inputTokens || 0),
      output: acc.output + (u.outputTokens || 0),
      cache: acc.cache + (u.cacheReadTokens || 0),
      cost: acc.cost + (u.costUsd || 0),
      contextUsed: acc.contextUsed || u.contextUsed,
      contextSize: acc.contextSize || u.contextSize,
    }),
    { input: 0, output: 0, cache: 0, cost: 0, contextUsed: undefined, contextSize: undefined }
  );

  const cacheRate = total.input > 0
    ? Math.round((total.cache / total.input) * 100)
    : 0;

  // Context 使用率（ACP 协议提供）
  const contextRate = total.contextSize && total.contextSize > 0
    ? Math.round(((total.contextUsed || 0) / total.contextSize) * 100)
    : 0;

  return (
    <div className="status-section token-section">
      <div className="status-section-header collapsible" onClick={() => setCollapsed(!collapsed)}>
        <span className="status-section-title">Token 统计</span>
        <RightOutlined className={`collapse-icon ${collapsed ? '' : 'expanded'}`} />
      </div>
      {!collapsed && (
        <>
          {/* Context 使用情况（ACP 协议提供） */}
          {total.contextSize && total.contextSize > 0 && (
            <div className="context-usage">
              <div className="context-usage-header">
                <span className="context-usage-label">Context 使用</span>
                <span className="context-usage-value">
                  {formatTokens(total.contextUsed || 0)} / {formatTokens(total.contextSize)}
                  <span className="context-usage-percent"> ({contextRate}%)</span>
                </span>
              </div>
              <Progress
                percent={contextRate}
                size="small"
                showInfo={false}
                strokeColor={contextRate > 80 ? '#ef4444' : contextRate > 60 ? '#f59e0b' : '#22c55e'}
              />
            </div>
          )}

          <div className="token-grid">
            <div className="token-item">
              <span className="token-label">输入</span>
              <AnimatedValue
                value={formatTokens(total.input)}
                className="token-value input"
              />
            </div>
            <div className="token-item">
              <span className="token-label">输出</span>
              <AnimatedValue
                value={formatTokens(total.output)}
                className="token-value output"
              />
            </div>
            <div className="token-item">
              <span className="token-label">缓存</span>
              <AnimatedValue
                value={formatTokens(total.cache)}
                className="token-value"
              />
            </div>
            <div className="token-item">
              <span className="token-label">成本</span>
              <AnimatedValue
                value={`$${total.cost.toFixed(4)}`}
                className="token-value cost"
              />
            </div>
          </div>

          {cacheRate > 0 && (
            <div className="cache-rate">
              <div className="cache-rate-header">
                <span className="cache-rate-label">缓存命中率</span>
                <span className="cache-rate-value">{cacheRate}%</span>
              </div>
              <Progress
                percent={cacheRate}
                size="small"
                showInfo={false}
                strokeColor="#22c55e"
              />
            </div>
          )}
        </>
      )}
    </div>
  );
};

export default TokenUsage;