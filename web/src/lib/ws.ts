type Handler = (topic: string, data: unknown) => void;

export class LiveSocket {
  private ws: WebSocket | null = null;
  private handlers = new Set<Handler>();
  private topics = new Set<string>();
  private timer: number | null = null;

  start() {
    this.connect();
  }

  stop() {
    if (this.timer) window.clearTimeout(this.timer);
    this.ws?.close();
    this.ws = null;
  }

  subscribe(topics: string[]) {
    topics.forEach((t) => this.topics.add(t));
    this.send({ type: "subscribe", topics });
  }

  unsubscribe(topics: string[]) {
    topics.forEach((t) => this.topics.delete(t));
    this.send({ type: "unsubscribe", topics });
  }

  on(handler: Handler) {
    this.handlers.add(handler);
    return () => this.handlers.delete(handler);
  }

  private connect() {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/api/ws`);
    this.ws = ws;
    ws.onopen = () => {
      if (this.topics.size) {
        this.send({ type: "subscribe", topics: [...this.topics] });
      }
    };
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data as string) as { type: string; topic?: string; data?: unknown };
        if (msg.type === "event" && msg.topic) {
          this.handlers.forEach((h) => h(msg.topic!, msg.data));
        }
      } catch {
        /* ignore */
      }
    };
    ws.onclose = () => {
      this.timer = window.setTimeout(() => this.connect(), 2000);
    };
  }

  private send(payload: unknown) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(payload));
    }
  }
}

export const live = new LiveSocket();
