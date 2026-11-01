export type TooltipRow = { label: string; value: string; color: string };

/**
 * Series labels can be guest names, which come from the cluster, so the
 * tooltip is built from text nodes and never from an HTML string.
 */
export function fillChartTooltip(tip: HTMLElement, title: string, rows: TooltipRow[]) {
  const doc = tip.ownerDocument;
  const el = (className: string) => {
    const node = doc.createElement("div");
    node.className = className;
    return node;
  };
  const heading = el("mb-1 text-[11px] text-ink-dim");
  heading.textContent = title;
  const lines = rows.map((row) => {
    const line = el("flex items-center justify-between gap-6");
    const name = el("flex items-center gap-1.5");
    const swatch = el("h-1.5 w-1.5 rounded-full");
    swatch.style.background = row.color;
    const label = doc.createElement("span");
    label.textContent = row.label;
    name.append(swatch, label);
    const value = el("font-mono");
    value.textContent = row.value;
    line.append(name, value);
    return line;
  });
  tip.replaceChildren(heading, ...lines);
}
