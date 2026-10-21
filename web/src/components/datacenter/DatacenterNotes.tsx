import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { MarkdownView } from "@/lib/markdown";
import { useCan } from "@/lib/perms";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

/** Datacenter notes (the cluster "description" option), editable in place. */
export function DatacenterNotes() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const opts = useQuery({ queryKey: ["dc-opts"], queryFn: api.dcOptions });
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const notes = opts.data ? String(opts.data.description ?? "") : "";

  return (
    <Card className="p-5">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold">Notes</h2>
        {writable && !editing && (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => {
              setDraft(notes);
              setError("");
              setEditing(true);
            }}
          >
            Edit
          </Button>
        )}
      </div>
      {editing ? (
        <div className="mt-3 space-y-3">
          <Textarea
            aria-label="Datacenter notes"
            className="min-h-32 font-mono text-[12px]"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Markdown: headings, lists, tables, links."
          />
          {error && <p className="text-xs text-danger">{error}</p>}
          <div className="flex gap-2">
            <Button
              size="sm"
              disabled={saving}
              onClick={async () => {
                setError("");
                setSaving(true);
                try {
                  await api.updateDCOptions({ description: draft });
                  setEditing(false);
                  void qc.invalidateQueries({ queryKey: ["dc-opts"] });
                } catch (e) {
                  setError(e instanceof Error ? e.message : "Save failed");
                } finally {
                  setSaving(false);
                }
              }}
            >
              {saving ? "Saving…" : "Save"}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setEditing(false)}>
              Cancel
            </Button>
          </div>
        </div>
      ) : notes.trim() ? (
        <div className="mt-2 max-h-72 overflow-y-auto text-sm">
          <MarkdownView text={notes} />
        </div>
      ) : (
        <p className="mt-2 text-[13px] text-ink-dim">
          {writable ? "Add runbooks, contacts, or maintenance windows for everyone who manages this cluster." : "No notes."}
        </p>
      )}
    </Card>
  );
}
