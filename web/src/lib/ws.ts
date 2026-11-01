type Handler = (topic: string, data: unknown) => void;

const RECONNECT_MS = 2000;
const SCOPE = "conn:";

/** Names a detail topic on one connection; node names and VMIDs repeat across clusters. */
export function connectionTopic(connection: string, detail: string) {
  return `${SCOPE}${connection}:${detail}`;
}

/** The detail part of a connection topic. */
export function topicDetail(topic: string) {
  if (!topic.startsWith(SCOPE)) return topic;
  return topic.slice(topic.indexOf(":", SCOPE.length) + 1);
}

export class LiveSocket {
  private ws: WebSocket | null = null;
  private handlers = new Set<Handler>();
  private topics = new Map<string, number>();
  private timer: ReturnType<typeof setTimeout> | null = null;
  private running = false;

  start() {
    if (this.running) return;
    this.running = true;
    this.connect();
  }

  /** Closes the socket for good, until the next start. Forgets every topic. */
  stop() {
    this.running = false;
    if (this.timer !== null) clearTimeout(this.timer);
    this.timer = null;
    const ws = this.ws;
    this.ws = null;
    ws?.close();
    this.topics.clear();
  }

  /** Topics are counted, so each subscriber must unsubscribe once. */
  subscribe(topics: string[]) {
    const added = topics.filter((t) => {
      const count = this.topics.get(t) ?? 0;
      this.topics.set(t, count + 1);
      return count === 0;
    });
    if (added.length) this.send({ type: "subscribe", topics: added });
  }

  unsubscribe(topics: string[]) {
    const removed = topics.filter((t) => {
      const count = this.topics.get(t);
      if (!count) return false;
      if (count > 1) {
        this.topics.set(t, count - 1);
        return false;
      }
      this.topics.delete(t);
      return true;
    });
    if (removed.length) this.send({ type: "unsubscribe", topics: removed });
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
      if (this.ws === ws && this.topics.size) {
        this.send({ type: "subscribe", topics: [...this.topics.keys()] });
      }
    };
    ws.onmessage = (ev) => {
      if (this.ws !== ws) return;
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
      // stop() lets go of the socket first, so this only runs for a dropped one.
      if (this.ws !== ws) return;
      this.ws = null;
      this.timer = setTimeout(() => {
        this.timer = null;
        this.connect();
      }, RECONNECT_MS);
    };
  }

  private send(payload: unknown) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(payload));
    }
  }
}

export const live = new LiveSocket();
