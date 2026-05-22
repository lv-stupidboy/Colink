// isdp/web/src/hooks/useWebSocket.ts
import { useEffect, useRef, useCallback, useState } from 'react';
import { WSMessage } from '@/types';
import { useChunkBatcher, Chunk } from './useChunkBatcher';

interface UseWebSocketOptions {
  onMessage?: (data: WSMessage) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
  reconnectInterval?: number;
}

export function useWebSocket(
  threadId: string | null,
  options: UseWebSocketOptions = {}
) {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout>>();
  const [connected, setConnected] = useState(false);
  const { onMessage, onConnect, onDisconnect, reconnectInterval = 3000 } = options;

  const onMessageRef = useRef(onMessage);
  const onConnectRef = useRef(onConnect);
  const onDisconnectRef = useRef(onDisconnect);

  useEffect(() => {
    onMessageRef.current = onMessage;
    onConnectRef.current = onConnect;
    onDisconnectRef.current = onDisconnect;
  }, [onMessage, onConnect, onDisconnect]);

  const handleChunksFlush = useCallback((chunks: Chunk[]) => {
    chunks.forEach((chunk) => {
      onMessageRef.current?.(chunk as unknown as WSMessage);
    });
  }, []);

  const { enqueue, flushImmediately } = useChunkBatcher({
    onFlush: handleChunksFlush,
  });

  useEffect(() => {
    if (!threadId) {
      if (wsRef.current) {
        console.log('[WebSocket] Disconnecting due to null threadId');
        flushImmediately();
        if (reconnectTimeoutRef.current) {
          clearTimeout(reconnectTimeoutRef.current);
        }
        wsRef.current.close();
        wsRef.current = null;
        setConnected(false);
      }
      return;
    }

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/v1/ws?threadId=${threadId}`;
    console.log('[WebSocket] Connecting to:', wsUrl);

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('[WebSocket] Connected successfully, threadId:', threadId);
      setConnected(true);
      onConnectRef.current?.();

      if (threadId) {
        send({
          type: 'recover_invocation_state',
          threadId,
        });
      }
    };

    ws.onmessage = (event) => {
      try {
        const data: WSMessage = JSON.parse(event.data);

        if (data.type === 'agent_output_chunk') {
          const blockType = data.payload?.type as string || 'text';
          enqueue({
            type: 'agent_output_chunk',
            payload: data.payload || {},
            chunkType: blockType as any,
          });
        } else {
          flushImmediately();
          onMessageRef.current?.(data);
        }
      } catch (e) {
        console.error('[WebSocket] Failed to parse message:', e);
      }
    };

    ws.onclose = () => {
      console.log('[WebSocket] Disconnected, threadId:', threadId);
      setConnected(false);
      onDisconnectRef.current?.();
      flushImmediately();
      if (wsRef.current === ws) {
        reconnectTimeoutRef.current = setTimeout(() => {
          if (wsRef.current === ws && threadId) {
            console.log('[WebSocket] Attempting reconnect...');
          }
        }, reconnectInterval);
      }
    };

    ws.onerror = (error) => {
      console.error('[WebSocket] Error:', error);
    };

    // ✅ 页面可见性变化时处理重连
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        if (threadId) {
          // 检查连接状态
          if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
            console.log('[WebSocket] Page visible, reconnecting...');
            // 清理旧连接
            if (wsRef.current) {
              wsRef.current.close();
            }
            // 创建新连接（通过触发 effect 重新执行）
            setConnected(false);
          } else {
            console.log('[WebSocket] Connection alive, no action needed');
          }
        }
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      console.log('[WebSocket] Cleaning up connection');
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      flushImmediately();
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      ws.close();
      wsRef.current = null;
    };
  }, [threadId, reconnectInterval, enqueue, flushImmediately]);

  const send = useCallback((data: object) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data));
    }
  }, []);

  return { send, connected };
}