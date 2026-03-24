import { useEffect, useRef, useCallback, useState } from "react";

interface WsMessage {
  type: string;
  channel: string;
  data: unknown;
  timestamp: string;
}

interface WsClientOptions {
  url?: string;
  reconnectDelay?: number;
  maxReconnectAttempts?: number;
}

class WebSocketManager {
  private ws: WebSocket | null = null;
  private subscriptions = new Map<string, Set<(data: unknown) => void>>();
  private reconnectAttempts = 0;
  private maxReconnectAttempts: number;
  private reconnectDelay: number;
  private url: string;
  private heartbeatInterval: NodeJS.Timeout | null = null;
  private token: string | null = null;

  constructor(options: WsClientOptions = {}) {
    this.url = options.url || process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8080";
    this.reconnectDelay = options.reconnectDelay || 1000;
    this.maxReconnectAttempts = options.maxReconnectAttempts || 10;
  }

  connect(token: string) {
    if (this.ws?.readyState === WebSocket.OPEN) return;

    this.token = token;
    const wsUrl = new URL(this.url);
    wsUrl.searchParams.set("token", token);

    this.ws = new WebSocket(wsUrl.toString());

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.startHeartbeat();
      // Resubscribe to all channels
      this.subscriptions.forEach((_, channel) => {
        this.send({ action: "subscribe", channel });
      });
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: WsMessage = JSON.parse(event.data);
        if (msg.type === "pong") return;

        const handlers = this.subscriptions.get(msg.channel);
        handlers?.forEach((handler) => handler(msg.data));
      } catch (error) {
        console.error("Failed to parse WebSocket message:", error);
      }
    };

    this.ws.onclose = () => {
      this.stopHeartbeat();
      this.tryReconnect();
    };

    this.ws.onerror = (error) => {
      console.error("WebSocket error:", error);
    };
  }

  subscribe<T = unknown>(channel: string, handler: (data: T) => void): () => void {
    if (!this.subscriptions.has(channel)) {
      this.subscriptions.set(channel, new Set());
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.send({ action: "subscribe", channel });
      }
    }
    this.subscriptions.get(channel)!.add(handler as (data: unknown) => void);

    // Return unsubscribe function
    return () => {
      const handlers = this.subscriptions.get(channel);
      if (handlers) {
        handlers.delete(handler as (data: unknown) => void);
        if (handlers.size === 0) {
          this.subscriptions.delete(channel);
          if (this.ws?.readyState === WebSocket.OPEN) {
            this.send({ action: "unsubscribe", channel });
          }
        }
      }
    };
  }

  send(data: Record<string, unknown>) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  private startHeartbeat() {
    this.heartbeatInterval = setInterval(() => {
      this.send({ type: "ping" });
    }, 30000);
  }

  private stopHeartbeat() {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  private tryReconnect() {
    if (!this.token) return;
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error("Max WebSocket reconnection attempts reached");
      return;
    }

    const delay = Math.min(this.reconnectDelay * Math.pow(2, this.reconnectAttempts), 30000);
    this.reconnectAttempts++;

    setTimeout(() => {
      if (this.token) {
        this.connect(this.token);
      }
    }, delay);
  }

  disconnect() {
    this.stopHeartbeat();
    this.ws?.close();
    this.ws = null;
    this.subscriptions.clear();
    this.token = null;
  }
}

// Singleton instance
let wsManager: WebSocketManager | null = null;

export function getWebSocketManager(): WebSocketManager {
  if (!wsManager) {
    wsManager = new WebSocketManager();
  }
  return wsManager;
}

/**
 * React hook for using WebSocket
 */
export function useWebSocket<T = unknown>(
  channel: string,
  onMessage: (data: T) => void,
  enabled: boolean = true
) {
  const wsRef = useRef<WebSocketManager | null>(null);
  const unsubscribeRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    if (!enabled) return;

    wsRef.current = getWebSocketManager();
    const ws = wsRef.current;

    // Get token from auth store
    const token = document.cookie
      .split("; ")
      .find((row) => row.startsWith("auth_token="))
      ?.split("=")[1];

    if (token) {
      ws.connect(token);
    }

    unsubscribeRef.current = ws.subscribe<T>(channel, onMessage);

    return () => {
      unsubscribeRef.current?.();
    };
  }, [channel, onMessage, enabled]);

  const sendMessage = useCallback((data: Record<string, unknown>) => {
    wsRef.current?.send(data);
  }, []);

  return { sendMessage };
}
