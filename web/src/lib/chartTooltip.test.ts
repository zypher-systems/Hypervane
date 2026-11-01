import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { fillChartTooltip } from "./chartTooltip.ts";

class FakeNode {
  className = "";
  style: { background?: string } = {};
  children: FakeNode[] = [];
  text = "";
  htmlWrites = 0;

  get textContent() {
    return this.text;
  }

  set textContent(value: string) {
    this.text = value;
  }

  set innerHTML(_value: string) {
    this.htmlWrites += 1;
  }

  append(...nodes: FakeNode[]) {
    this.children.push(...nodes);
  }

  replaceChildren(...nodes: FakeNode[]) {
    this.children = nodes;
  }

  get ownerDocument() {
    return { createElement: () => new FakeNode() };
  }
}

function collect(node: FakeNode): FakeNode[] {
  return [node, ...node.children.flatMap(collect)];
}

describe("chart tooltip", () => {
  it("keeps a hostile guest name as plain text", () => {
    const hostile = '<img src=x onerror="alert(1)">';
    const tip = new FakeNode();
    fillChartTooltip(tip as unknown as HTMLElement, "12:00", [{ label: hostile, value: "42%", color: "#fff" }]);
    const nodes = collect(tip);
    assert.ok(nodes.some((n) => n.text === hostile));
    assert.equal(nodes.reduce((sum, n) => sum + n.htmlWrites, 0), 0);
  });

  it("replaces the last hover instead of adding to it", () => {
    const tip = new FakeNode();
    const rows = [{ label: "a", value: "1", color: "#fff" }];
    fillChartTooltip(tip as unknown as HTMLElement, "12:00", rows);
    fillChartTooltip(tip as unknown as HTMLElement, "12:01", rows);
    assert.equal(tip.children.length, 2);
    assert.equal(tip.children[0].text, "12:01");
  });
});
