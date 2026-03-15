import { useEffect, useRef, useState } from 'react';

interface WSMessage {
  type: string;
  threadId: string;
  timestamp: number;
  payload: Record<string, unknown>;
}

interface UseWebSocketOptions {
  threadId: string | null;
  onMessage?: (message: WSMessage) => void;
}

export function useWebSocket({ threadId, onMessage }: UseWebSocketOptions) {
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const onMessageRef = useRef(onMessage);

  // 同步回调函数到 ref
  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  // 当 threadId 改变时连接 WebSocket
  useEffect(() => {
    if (!threadId) {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      setConnected(false);
      return;
    }

    // 清理之前的连接
    if (wsRef.current) {
      wsRef.current.close();
    }

    // 构建 WebSocket URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws?threadId=${threadId}`;

    console.log('[WebSocket] Connecting to:', wsUrl);

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('[WebSocket] Connected to thread:', threadId);
      setConnected(true);
    };

    ws.onmessage = (event) => {
      try {
        const message: WSMessage = JSON.parse(event.data);
        console.log('[WebSocket] Received message:', message.type);
        onMessageRef.current?.(message);
      } catch (err) {
        console.error('[WebSocket] Failed to parse message:', err);
      }
    };

    ws.onclose = () => {
      console.log('[WebSocket] Disconnected');
      setConnected(false);
    };

    ws.onerror = (err) => {
      console.error('[WebSocket] Error:', err);
    };

    wsRef.current = ws;

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [threadId]);

  return { connected };
}

// 辅助函数：等待 WebSocket 连接
export function waitForWebSocketConnect(threadId: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws?threadId=${threadId}`;

    console.log('[WebSocket] Connecting (wait):', wsUrl);

    const ws = new WebSocket(wsUrl);
    let resolved = false;

    ws.onopen = () => {
      if (!resolved) {
        resolved = true;
        console.log('[WebSocket] Connected (wait)');
        ws.close(); // 连接成功后关闭，让 useWebSocket 重新连接
        resolve();
      }
    };

    ws.onerror = (err) => {
      if (!resolved) {
        resolved = true;
        reject(err);
      }
    };

    // 超时保护
    setTimeout(() => {
      if (!resolved) {
        resolved = true;
        ws.close();
        reject(new Error('WebSocket connection timeout'));
      }
    }, 5000);
  });
}

export default useWebSocket;