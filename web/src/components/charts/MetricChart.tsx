import { bytes, bytesRate, ops, USAGE_OK, usageHex } from "@/lib/format";
import { useEffect, useRef, useState } from "react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";

type Series = { key: string; label: string; color: string; scale?: (v: number) => number };
export type ChartFormat = "percent" | "bytes" | "rate" | "ops" | "raw";


function thresholdPaths(u: uPlot, seriesIdx: number, idx0: number, idx1: number): uPlot.Series.Paths {
  const xs = u.data[0];
  const ys = u.data[seriesIdx];
  const strokes = new Map<string, Path2D>();
  const add = (color: string, x1: number, y1: number, x2: number, y2: number) => {
    let path = strokes.get(color);
    if (!path) {
      path = new Path2D();
      strokes.set(color, path);
    }
    path.moveTo(x1, y1);
    path.lineTo(x2, y2);
  };
  const pos = (i: number) => ({
    x: u.valToPos(Number(xs[i]), "x", true),
    y: u.valToPos(Number(ys[i]), "y", true),
    v: Number(ys[i]),
  });
  for (let i = idx0; i < idx1; i++) {
    if (ys[i] == null || ys[i + 1] == null) continue;
    const a = pos(i);
    const b = pos(i + 1);
    if (!Number.isFinite(a.v) || !Number.isFinite(b.v)) continue;
    const cuts = [35, 70].filter((t) => (a.v - t) * (b.v - t) < 0);
    const pts = [a];
    for (const t of cuts) {
      const f = (t - a.v) / (b.v - a.v);
      pts.push({ x: a.x + (b.x - a.x) * f, y: a.y + (b.y - a.y) * f, v: t });
    }
    pts.push(b);
    for (let p = 0; p < pts.length - 1; p++) {
      add(usageHex((pts[p].v + pts[p + 1].v) / 2), pts[p].x, pts[p].y, pts[p + 1].x, pts[p + 1].y);
    }
  }
  return { stroke: strokes, fill: null, clip: null };
}

function fmtY(v: number, format: ChartFormat) {
  if (!Number.isFinite(v)) return "—";
  if (format === "percent") return `${v < 10 ? v.toFixed(1) : v.toFixed(0)}%`;
  if (format === "bytes") return bytes(v);
  if (format === "rate") return bytesRate(v);
  if (format === "ops") return ops(v);
  if (Math.abs(v) >= 1000) return v.toLocaleString(undefined, { maximumFractionDigits: 0 });
  return String(Math.round(v * 10) / 10);
}

function fmtTime(t: number) {
  const ms = t > 1e12 ? t : t * 1000;
  return new Date(ms).toLocaleString();
}

function alignData(points: Record<string, number>[], series: Series[]): uPlot.AlignedData {
  const times = points.map((p) => Number(p.time ?? p.ts ?? 0));
  return [
    times,
    ...series.map((s) => points.map((p) => {
      const v = Number(p[s.key]);
      if (Number.isNaN(v)) return null as unknown as number;
      return s.scale ? s.scale(v) : v;
    })),
  ];
}

export function MetricChart({
  points,
  series,
  height = 200,
  format = "raw",
  tipSide = "right",
  compact = false,
  threshold = false,
}: {
  points: Record<string, number>[];
  series: Series[];
  height?: number;
  format?: ChartFormat;
  tipSide?: "left" | "right";
  compact?: boolean;
  threshold?: boolean;
}) {
  const wrap = useRef<HTMLDivElement>(null);
  const tipEl = useRef<HTMLDivElement>(null);
  const plot = useRef<uPlot | null>(null);
  const pointsRef = useRef(points);
  const seriesRef = useRef(series);
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  const seriesKey = series.map((s) => `${s.key}:${s.color}`).join("|");
  pointsRef.current = points;
  seriesRef.current = series;

  useEffect(() => {
    const wrapEl = wrap.current;
    if (!host || !wrapEl) return;
    const currentSeries = seriesRef.current;
    const data = alignData(pointsRef.current, currentSeries);
    const opts: uPlot.Options = {
      width: wrapEl.clientWidth || 640,
      height,
      legend: { show: false },
      cursor: {
        drag: { x: false, y: false },
        points: { show: false },
      },
      scales: {
        x: { time: true },
        y: format === "percent"
          ? { range: [0, 100] }
          : { range: (_u, _min, max) => [0, max > 0 ? max * 1.08 : 1] },
      },
      padding: compact ? [6, 8, 6, 8] : undefined,
      axes: compact
        ? [
            { show: false },
            { show: false },
          ]
        : [
            { stroke: "#5a6270", grid: { stroke: "#20242e" } },
            {
              stroke: "#5a6270",
              grid: { stroke: "#20242e" },
              size: format === "bytes" || format === "rate" ? 70 : 44,
              values: (_u, splits) => splits.map((v) => fmtY(v, format)),
            },
          ],
      series: [
        {},
        ...currentSeries.map((s) => ({
          label: s.label,
          stroke: threshold ? USAGE_OK : s.color,
          width: compact ? 1.8 : 1.6,
          points: { show: false },
          fill: threshold || s.color.length !== 7 ? undefined : `${s.color}26`,
          paths: threshold ? thresholdPaths : undefined,
        })),
      ],
      hooks: {
        setCursor: [
          (u) => {
            const tip = tipEl.current;
            if (!tip) return;
            const { idx, left, top } = u.cursor;
            if (idx == null || left == null || top == null || left < 0) {
              tip.hidden = true;
              return;
            }
            const t = u.data[0][idx];
            if (t == null) {
              tip.hidden = true;
              return;
            }
            const rows = seriesRef.current.map((s, i) => {
              const v = u.data[i + 1][idx];
              const swatch = v == null ? s.color : threshold ? usageHex(Number(v)) : s.color;
              return `<div class="flex items-center justify-between gap-6">
                <span class="flex items-center gap-1.5"><span class="h-1.5 w-1.5 rounded-full" style="background:${swatch}"></span>${s.label}</span>
                <span class="font-mono">${v == null ? "—" : fmtY(Number(v), format)}</span>
              </div>`;
            }).join("");
            tip.innerHTML = `<div class="mb-1 text-[11px] text-ink-dim">${fmtTime(Number(t))}</div>${rows}`;
            tip.hidden = false;
            const origin = u.over.getBoundingClientRect();
            const tw = tip.offsetWidth;
            const th = tip.offsetHeight;
            tip.style.left = tipSide === "left"
              ? `${origin.left + left - tw - 14}px`
              : `${origin.left + left + 14}px`;
            tip.style.top = `${origin.top + top - th - 14}px`;
          },
        ],
      },
    };
    plot.current?.destroy();
    plot.current = new uPlot(opts, data, host);
    const onResize = () => {
      if (wrap.current && plot.current) {
        plot.current.setSize({ width: wrap.current.clientWidth, height });
      }
    };
    const ro = new ResizeObserver(onResize);
    ro.observe(wrapEl);
    window.addEventListener("resize", onResize);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", onResize);
      plot.current?.destroy();
      plot.current = null;
    };
  }, [host, height, format, tipSide, seriesKey, compact, threshold]);

  useEffect(() => {
    if (!plot.current || !points.length) return;
    plot.current.setData(alignData(points, series));
  }, [points, series, seriesKey]);

  if (!points.length) {
    return (
      <div className="flex items-center justify-center text-sm text-ink-dim" style={{ height }}>
        No samples yet
      </div>
    );
  }
  return (
    <div ref={wrap} className="relative w-full overflow-visible">
      <div ref={setHost} className="w-full" />
      <div
        ref={tipEl}
        hidden
        className="hv-glass pointer-events-none fixed z-50 rounded-md border px-2.5 py-2 text-xs text-ink shadow-lg"
      />
    </div>
  );
}
