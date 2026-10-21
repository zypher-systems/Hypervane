import { MetricChart } from "@/components/charts/MetricChart";
import { Card } from "@/components/ui/card";
import { Field, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { CHART_CPU, CHART_MEM } from "@/lib/chartColors";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";

export function HistoryPage() {
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const guests = (summary.data?.resources ?? []).filter((r) => r.type === "qemu" || r.type === "lxc");
  const [key, setKey] = useState("");
  const selected = key || guests[0]?.id || "";
  const guest = guests.find((g) => g.id === selected);
  const hist = useQuery({
    queryKey: ["metric-history", selected],
    queryFn: () => api.metricHistory(selected),
    enabled: !!selected,
    refetchInterval: 30000,
  });
  const iops = useQuery({
    queryKey: ["history-iops", guest?.vmid, guest?.node, guest?.type],
    queryFn: () => api.guestIoRrd(guest!.vmid!, "year", guest?.node, guest?.type),
    enabled: !!guest?.vmid,
  });
  const points = useMemo(
    () =>
      (hist.data?.points ?? []).map((p) => ({
        time: Math.floor(new Date(p.time).getTime() / 1000),
        cpu: Number(p.cpu) * 100,
        mem: p.maxmem ? Math.min(100, (100 * p.mem) / p.maxmem) : 0,
      })),
    [hist.data],
  );
  const iopsPoints = useMemo(
    () =>
      (iops.data?.iops ?? []).map((p) => ({
        time: Number(p.time),
        rd_requests: Number(p.rd_requests ?? 0),
        wr_requests: Number(p.wr_requests ?? 0),
      })),
    [iops.data],
  );

  return (
    <div className="space-y-6">
      <DatacenterHeader />
      <header>
        <h2 className="text-base font-semibold">Long history</h2>
        <p className="text-sm text-ink-dim">CPU, memory, and IOPS from Hypervane's own long-term samples.</p>
      </header>
      <Field label="Guest">
        <Select value={selected} onChange={(e) => setKey(e.target.value)}>
          {guests.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name || `VM ${g.vmid}`}
            </option>
          ))}
        </Select>
      </Field>
      <Card className="p-5">
        <p className="mb-2 text-xs font-medium text-ink-dim">CPU / Memory</p>
        <MetricChart
          points={points}
          format="percent"
          height={240}
          series={[
            { key: "cpu", label: "CPU", color: CHART_CPU },
            { key: "mem", label: "Memory", color: CHART_MEM },
          ]}
        />
      </Card>
      <Card className="p-5">
        <p className="mb-2 text-xs font-medium text-ink-dim">IOPS</p>
        <MetricChart
          points={iopsPoints}
          format="ops"
          height={240}
          series={[
            { key: "rd_requests", label: "Read", color: CHART_MEM },
            { key: "wr_requests", label: "Write", color: CHART_CPU },
          ]}
        />
      </Card>
    </div>
  );
}
