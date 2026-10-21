import { LogPanel, OptionsBar, RuleTable } from "@/components/firewall/shared";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import { useQuery, useQueryClient } from "@tanstack/react-query";

export function VNetFirewall({ vnet }: { vnet: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate") || can("/sdn", "Sys.Modify");
  const opts = useQuery({ queryKey: ["fw-vnet-opts", vnet], queryFn: () => api.fwVNetOptions(vnet) });
  const rules = useQuery({ queryKey: ["fw-vnet-rules", vnet], queryFn: () => api.fwVNetRules(vnet) });
  const log = useQuery({
    queryKey: ["fw-vnet-log", vnet],
    queryFn: () => api.fwVNetLog(vnet),
    refetchInterval: 15000,
  });
  const macros = useQuery({ queryKey: ["fw-macros"], queryFn: api.fwMacros });
  const groups = useQuery({ queryKey: ["fw-groups"], queryFn: api.fwGroups });

  return (
    <div className="space-y-4">
      {opts.data && (
        <OptionsBar
          key={opts.data.digest || vnet}
          opts={opts.data}
          writable={writable}
          onSave={async (body) => {
            await api.updateFWVNetOptions(vnet, body);
            void qc.invalidateQueries({ queryKey: ["fw-vnet-opts", vnet] });
          }}
        />
      )}
      <RuleTable
        rules={rules.data ?? []}
        writable={writable}
        macros={macros.data}
        groups={(groups.data ?? []).map((g) => g.group)}
        onCreate={async (body) => {
          await api.createFWVNetRule(vnet, body);
          void qc.invalidateQueries({ queryKey: ["fw-vnet-rules", vnet] });
        }}
        onUpdate={async (pos, body) => {
          await api.updateFWVNetRule(vnet, pos, body);
          void qc.invalidateQueries({ queryKey: ["fw-vnet-rules", vnet] });
        }}
        onDelete={async (pos, digest) => {
          await api.deleteFWVNetRule(vnet, pos, digest);
          void qc.invalidateQueries({ queryKey: ["fw-vnet-rules", vnet] });
        }}
      />
      <LogPanel lines={log.data ?? []} />
    </div>
  );
}
