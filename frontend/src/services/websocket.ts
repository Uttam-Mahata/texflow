import type { WebSocketMessage, UserPresence } from '@/types';

const WS_BASE_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8082';

export type WebSocketEventHandler = (message: WebSocketMessage) => void;
export type PresenceEventHandler = (users: Map<string, UserPresence>) => void;

class WebSocketService {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private heartbeatInterval: NodeJS.Timeout | null = null;
  private messageHandlers: Set<WebSocketEventHandler> = new Set();
  private presenceHandlers: Set<PresenceEventHandler> = new Set();
  private connectedUsers: Map<string, UserPresence> = new Map();
  private projectId: string | null = null;

  connect(projectId: string): Promise<void> {
    return new Promise((resolve, reject) => {
      this.projectId = projectId;
      const token = localStorage.getItem('access_token');

      if (!token) {
        reject(new Error('No access token found'));
        return;
      }

      const wsUrl = `${WS_BASE_URL}/ws?project_id=${projectId}&token=${token}`;
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('WebSocket connected');
        this.reconnectAttempts = 0;
        this.startHeartbeat();
        resolve();
      };

      this.ws.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          this.handleMessage(message);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        reject(error);
      };

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.stopHeartbeat();
        this.attemptReconnect();
      };
    });
  }

  disconnect(): void {
    this.stopHeartbeat();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.connectedUsers.clear();
    this.notifyPresenceHandlers();
  }

  send(message: Partial<WebSocketMessage>): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      console.warn('WebSocket is not open. Message not sent:', message);
    }
  }

  onMessage(handler: WebSocketEventHandler): () => void {
    this.messageHandlers.add(handler);
    return () => this.messageHandlers.delete(handler);
  }

  onPresence(handler: PresenceEventHandler): () => void {
    this.presenceHandlers.add(handler);
    // Immediately notify with current state
    handler(new Map(this.connectedUsers));
    return () => this.presenceHandlers.delete(handler);
  }

  private handleMessage(message: WebSocketMessage): void {
    // Update presence
    if (message.type === 'user_joined') {
      this.connectedUsers.set(message.user_id, {
        user_id: message.user_id,
        user_name: message.user_name,
        color: message.payload.color,
      });
      this.notifyPresenceHandlers();
    } else if (message.type === 'user_left') {
      this.connectedUsers.delete(message.user_id);
      this.notifyPresenceHandlers();
    } else if (message.type === 'cursor_update') {
      const user = this.connectedUsers.get(message.user_id);
      if (user) {
        user.cursor = message.payload.cursor;
        this.notifyPresenceHandlers();
      }
    } else if (message.type === 'selection_update') {
      const user = this.connectedUsers.get(message.user_id);
      if (user) {
        user.selection = message.payload.selection;
        this.notifyPresenceHandlers();
      }
    }

    // Notify all message handlers
    this.messageHandlers.forEach((handler) => handler(message));
  }

  private notifyPresenceHandlers(): void {
    this.presenceHandlers.forEach((handler) => handler(new Map(this.connectedUsers)));
  }

  private startHeartbeat(): void {
    this.heartbeatInterval = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }));
      }
    }, 30000); // Every 30 seconds
  }

  private stopHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('Max reconnection attempts reached');
      return;
    }

    if (!this.projectId) {
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);

    console.log(`Attempting to reconnect in ${delay}ms (attempt ${this.reconnectAttempts})`);

    setTimeout(() => {
      if (this.projectId) {
        this.connect(this.projectId).catch((error) => {
          console.error('Reconnection failed:', error);
        });
      }
    }, delay);
  }

  getConnectedUsers(): Map<string, UserPresence> {
    return new Map(this.connectedUsers);
  }

  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }
}

export const websocketService = new WebSocketService();
export default websocketService;
