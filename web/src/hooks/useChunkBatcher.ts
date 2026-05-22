import { useRef, useCallback, useEffect } from 'react';

export type ChunkType = 'text' | 'thinking' | 'tool_use' | 'tool_result' | 'question';

export interface Chunk {
  type: string;
  payload: Record<string, unknown>;
  chunkType?: ChunkType;
}

export interface ChunkBatcherOptions {
  flushInterval?: number;
  onFlush: (chunks: Chunk[]) => void;
}

const DEFAULT_INTERVALS: Record<ChunkType, number> = {
  text: 100,
  thinking: 200,
  tool_use: 0, // 立即处理
  tool_result: 0,
  question: 0,
};

export function useChunkBatcher(options: ChunkBatcherOptions) {
  const { flushInterval, onFlush } = options;

  const bufferRef = useRef<Chunk[]>([]);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onFlushRef = useRef(onFlush);

  useEffect(() => {
    onFlushRef.current = onFlush;
  }, [onFlush]);

  const flushImmediately = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    if (bufferRef.current.length > 0) {
      onFlushRef.current(bufferRef.current);
      bufferRef.current = [];
    }
  }, []);

  const enqueue = useCallback((chunk: Chunk) => {
    bufferRef.current.push(chunk);

    const chunkType = (chunk.chunkType || 'text') as ChunkType;
    const interval = flushInterval ?? DEFAULT_INTERVALS[chunkType] ?? DEFAULT_INTERVALS.text;

    if (interval <= 0) {
      flushImmediately();
      return;
    }

    if (!timerRef.current) {
      timerRef.current = setTimeout(() => {
        if (bufferRef.current.length > 0) {
          onFlushRef.current(bufferRef.current);
          bufferRef.current = [];
        }
        timerRef.current = null;
      }, interval);
    }
  }, [flushInterval, flushImmediately]);

  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      if (bufferRef.current.length > 0) {
        onFlushRef.current(bufferRef.current);
      }
    };
  }, []);

  return { enqueue, flushImmediately };
}
