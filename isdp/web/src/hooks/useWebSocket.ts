// isdp/web/src/hooks/useWebSocket.ts
import { useEffect, useRef, useCallback, useState } from 'react';
import { WSMessage } from '@/types';

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

  // 使用 ref 存储 onMessage，避免依赖变化导致重连
  const onMessageRef = useRef(onMessage);
  const onConnectRef = useRef(onConnect);
  const onDisconnectRef = useRef(onDisconnect);

  useEffect(() => {
    onMessageRef.current = onMessage;
    onConnectRef.current = onConnect;
    onDisconnectRef.current = onDisconnect;
  }, [onMessage, onConnect, onDisconnect]);

  useEffect(() => {
    if (!threadId) {
      // threadId 为空时断开连接
      if (wsRef.current) {
        console.log('[WebSocket] Disconnecting due to null threadId');
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

      // 请求恢复未完成的 invocation 内容（后台执行支持）
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
        console.log('[WebSocket] Received message:', data.type);
        onMessageRef.current?.(data);
      } catch (e) {
        console.error('[WebSocket] Failed to parse message:', e);
      }
    };

    ws.onclose = () => {
      console.log('[WebSocket] Disconnected, threadId:', threadId);
      setConnected(false);
      onDisconnectRef.current?.();
      // 只有当 threadId 仍然存在时才重连
      if (wsRef.current === ws) {
        reconnectTimeoutRef.current = setTimeout(() => {
          if (wsRef.current === ws && threadId) {
            console.log('[WebSocket] Attempting reconnect...');
            // 重新连接需要通过触发 effect 重新执行
          }
        }, reconnectInterval);
      }
    };

    ws.onerror = (error) => {
      console.error('[WebSocket] Error:', error);
    };

    return () => {
      console.log('[WebSocket] Cleaning up connection');
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      ws.close();
      wsRef.current = null;
    };
  }, [threadId, reconnectInterval]);

  const send = useCallback((data: object) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data));
    }
  }, []);

  return { send, connected };
}