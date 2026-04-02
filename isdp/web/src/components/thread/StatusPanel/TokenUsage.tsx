import React, { useState, useEffect, useRef } from 'react';
import { Progress } from 'antd';
import type { TokenUsage as TokenUsageType } from '@/types/status';

interface Props {
  usage: Record<string, TokenUsageType>;
  totalUsage?: {
    input: number;
    output: number;
    cache: number;
    cost: number;
  };
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

export const TokenUsage: React.FC<Props> = ({ usage, totalUsage }) => {
  // 使用传入的 totalUsage 或自行计算
  const total = totalUsage || Object.values(usage).reduce(
    (acc, u) => ({
      input: acc.input + (u.inputTokens || 0),
      output: acc.output + (u.outputTokens || 0),
      cache: acc.cache + (u.cacheReadTokens || 0),
      cost: acc.cost + (u.costUsd || 0),
    }),
    { input: 0, output: 0, cache: 0, cost: 0 }
  );

  const cacheRate = total.input > 0
    ? Math.round((total.cache / total.input) * 100)
    : 0;

  return (
    <div className="status-section">
      <div className="status-section-title">Token 统计</div>
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
    </div>
  );
};

export default TokenUsage;