import { useEffect, useRef } from "react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import { CHART_PRIMARY } from "@/lib/chartColors";

export function Sparkline({
  values,
  color = CHART_PRIMARY,
  height = 36,
}: {
  values: number[];
  color?: string;
  height?: number;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const plot = useRef<uPlot | null>(null);

  useEffect(() => {
    if (!ref.current) return;
    const xs = values.map((_, i) => i);
    const opts: uPlot.Options = {
      width: ref.current.clientWidth || 160,
      height,
      cursor: { show: false },
      legend: { show: false },
      scales: { x: { time: false } },
      axes: [{ show: false }, { show: false }],
      series: [{}, { stroke: color, width: 1.5, fill: color + "22" }],
    };
    plot.current?.destroy();
    plot.current = new uPlot(opts, [xs, values], ref.current);
    return () => {
      plot.current?.destroy();
      plot.current = null;
    };
  }, [values, color, height]);

  return <div ref={ref} className="h-9 w-full" />;
}
