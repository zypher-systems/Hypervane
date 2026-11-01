import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it, mock } from "node:test";
import { LiveSocket, connectionTopic, topicDetail } from "./ws.ts";

class FakeSocket {
  static OPEN = 1;
  static made: FakeSocket[] = [];
  readyState = 0;
  sent: { type: string; topics: string[] }[] = [];
  closed = false;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;

  constructor() {
    FakeSocket.made.push(this);
  }

  send(raw: string) {
    this.sent.push(JSON.parse(raw));
  }

  close() {
    this.closed = true;
    this.readyState = 3;
  }

  open() {
    this.readyState = FakeSocket.OPEN;
    this.onopen?.();
  }

  drop() {
    this.readyState = 3;
    this.onclose?.();
  }

  emit(topic: string, data: unknown) {
    this.onmessage?.({ data: JSON.stringify({ type: "event", topic, data }) });
  }
}

const globals = globalThis as unknown as { WebSocket: unknown; location: unknown };
const saved = { WebSocket: globals.WebSocket, location: globals.location };

function latest() {
  return FakeSocket.made[FakeSocket.made.length - 1];
}

function openSocket() {
  const live = new LiveSocket();
  live.start();
  latest().open();
  return live;
}

describe("LiveSocket", () => {
  beforeEach(() => {
    FakeSocket.made = [];
    globals.WebSocket = FakeSocket;
    Object.defineProperty(globalThis, "location", {
      configurable: true,
      value: { protocol: "http:", host: "hv.test" },
    });
    mock.timers.enable({ apis: ["setTimeout"] });
  });

  afterEach(() => {
    mock.timers.reset();
    globals.WebSocket = saved.WebSocket;
    Object.defineProperty(globalThis, "location", { configurable: true, value: saved.location });
  });

  it("keeps a shared topic until its last subscriber leaves", () => {
    const live = openSocket();
    const socket = latest();
    live.subscribe(["task:1"]);
    live.subscribe(["task:1"]);
    assert.deepEqual(socket.sent, [{ type: "subscribe", topics: ["task:1"] }]);

    live.unsubscribe(["task:1"]);
    assert.equal(socket.sent.length, 1);

    live.unsubscribe(["task:1"]);
    assert.deepEqual(socket.sent[1], { type: "unsubscribe", topics: ["task:1"] });
  });

  it("ignores an unsubscribe with no subscriber", () => {
    const live = openSocket();
    live.unsubscribe(["task:1"]);
    assert.deepEqual(latest().sent, []);
  });

  it("reconnects after a dropped socket and resubscribes once per topic", () => {
    const live = openSocket();
    live.subscribe(["a"]);
    live.subscribe(["a", "b"]);
    latest().drop();
    assert.equal(FakeSocket.made.length, 1);
    mock.timers.tick(2000);
    assert.equal(FakeSocket.made.length, 2);
    latest().open();
    assert.deepEqual(latest().sent, [{ type: "subscribe", topics: ["a", "b"] }]);
  });

  it("stays closed after stop", () => {
    const live = openSocket();
    const socket = latest();
    live.stop();
    assert.equal(socket.closed, true);
    socket.drop();
    mock.timers.tick(10_000);
    assert.equal(FakeSocket.made.length, 1);
  });

  it("cancels a pending reconnect on stop", () => {
    const live = openSocket();
    latest().drop();
    live.stop();
    mock.timers.tick(10_000);
    assert.equal(FakeSocket.made.length, 1);
  });

  it("does not stack sockets when started twice", () => {
    const live = new LiveSocket();
    live.start();
    live.start();
    assert.equal(FakeSocket.made.length, 1);
  });

  it("hears nothing from the last session after a restart", () => {
    const live = openSocket();
    const first = latest();
    live.subscribe(["task:old"]);
    const seen: string[] = [];
    live.on((topic) => seen.push(topic));

    live.stop();
    live.start();
    const second = latest();
    second.open();

    first.emit("resources", {});
    assert.deepEqual(seen, []);
    assert.equal(first.closed, true);
    assert.deepEqual(second.sent, []);

    second.emit("resources", {});
    assert.deepEqual(seen, ["resources"]);
  });

  it("tolerates a subscriber leaving after stop", () => {
    const live = openSocket();
    live.subscribe(["a"]);
    live.stop();
    live.unsubscribe(["a"]);
    live.start();
    latest().open();
    assert.deepEqual(latest().sent, []);
  });
});

describe("connection topics", () => {
  it("round-trip through the detail helper", () => {
    const upid = "UPID:pve1:0001:0002:0003:qmstart:100:root@pam:";
    const topic = connectionTopic("3f1c-a", `task:${upid}`);
    assert.equal(topic, `conn:3f1c-a:task:${upid}`);
    assert.equal(topicDetail(topic), `task:${upid}`);
    assert.equal(topicDetail("resources"), "resources");
  });
});
