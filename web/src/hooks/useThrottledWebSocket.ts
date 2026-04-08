// isdp/web/src/hooks/useThrottledWebSocket.ts
import { useRef, useCallback, useEffect } from 'react';

interface ThrottleOptions {
  delay: number; // 节流延迟（毫秒）
  maxBatchSize?: number; // 最大批量大小，超过则立即发送
}

interface StreamChunk {
  invocationId: string;
  chunk: string;
  agentId: string;
  agentName?: string;
}

/**
 * WebSocket 流式消息节流 Hook
 *
 * 将高频的流式 chunk 消息批量处理，减少状态更新频率
 * 参考 clowder-ai 的做法：流式内容直接 append 到当前消息
 */
export function useThrottledWebSocket(
  onBatch: (chunks: StreamChunk[]) => void,
  options: ThrottleOptions = { delay: 50, maxBatchSize: 10 }
) {
  const batchRef = useRef<StreamChunk[]>([]);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const flushRef = useRef<() => void>(() => {});

  // 立即发送批量
  const flush = useCallback(() => {
    if (batchRef.current.length > 0) {
      const chunksToSend = [...batchRef.current];
      batchRef.current = [];
      onBatch(chunksToSend);
    }
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
  }, [onBatch]);

  // 更新 flushRef（避免 useCallback 闭包问题）
  flushRef.current = flush;

  // 添加 chunk 到批量
  const addChunk = useCallback((chunk: StreamChunk) => {
    batchRef.current.push(chunk);

    // 超过最大批量大小，立即发送
    if (options.maxBatchSize && batchRef.current.length >= options.maxBatchSize) {
      flushRef.current();
      return;
    }

    // 设置节流定时器
    if (!timeoutRef.current) {
      timeoutRef.current = setTimeout(() => {
        flushRef.current();
      }, options.delay);
    }
  }, [options.delay, options.maxBatchSize]);

  // 清理定时器
  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  // 强制刷新（用于消息结束时）
  const forceFlush = useCallback(() => {
    flushRef.current();
  }, []);

  return { addChunk, forceFlush };
}