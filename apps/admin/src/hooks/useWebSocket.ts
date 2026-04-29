import { useEffect, useRef, useState, useCallback } from "react";
import { useAuthStore } from "@/stores/authStore";

export interface WSMessage {
  topic: string;
  payload: unknown;
  timestamp: string;
}

interface UseWebSocketOptions {
  url: string;
  onMessage?: (msg: WSMessage) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
}

export function useWebSocket(options: UseWebSocketOptions) {
  const {
    url,
    onMessage,
    onConnect,
    onDisconnect,
    reconnectInterval = 5000,
    maxReconnectAttempts = 10,
  } = options;

  const [isConnected, setIsConnected] = useState(false);
  const [connectionAttempts, setConnectionAttempts] = useState(0);
  const esRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  const connect = useCallback(() => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    // Use http(s) for SSE instead of ws:// — the Vite proxy forwards HTTP to the backend
    const baseUrl = url.replace(/^ws/, "http");
    const esUrl = `${baseUrl}?token=${encodeURIComponent(token)}`;

    const es = new EventSource(esUrl);
    esRef.current = es;

    es.onopen = () => {
      setIsConnected(true);
      setConnectionAttempts(0);
      onConnect?.();
    };

    es.onmessage = (event) => {
      try {
        // Backend sends "data: {json}\n\n" — EventSource strips the "data: " prefix
        const msg = JSON.parse(event.data) as WSMessage;
        onMessageRef.current?.(msg);
      } catch {
        // ignore non-JSON messages
      }
    };

    es.onerror = () => {
      setIsConnected(false);
      esRef.current = null;
      onDisconnect?.();

      const attempt = connectionAttempts + 1;
      if (attempt <= maxReconnectAttempts) {
        setConnectionAttempts(attempt);
        const delay = Math.min(
          reconnectInterval * Math.pow(1.5, attempt - 1),
          60000,
        );
        reconnectTimerRef.current = setTimeout(() => {
          connect();
        }, delay);
      }
    };
  }, [url, onConnect, onDisconnect, reconnectInterval, maxReconnectAttempts, connectionAttempts]);

  const disconnect = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
    }
    if (esRef.current) {
      esRef.current.close();
      esRef.current = null;
    }
    setIsConnected(false);
  }, []);

  const send = useCallback((_data: unknown) => {
    // SSE is server→client only; send is a no-op here.
    // If bidirectional messaging is needed later, switch to WebSocket.
  }, []);

  useEffect(() => {
    connect();
    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  return { isConnected, send, connect, disconnect, connectionAttempts };
}
