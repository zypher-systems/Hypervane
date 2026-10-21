import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { formatTags, parseTags, TagChip } from "@/components/ui/tags";
import { Plus, X } from "lucide-react";
import { useState } from "react";

/**
 * Typed guest tag editor: chip list with add/remove. Stores back to PVE's
 * semicolon-separated `tags` string. Keeps ordering stable.
 */
export function TagEditor({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const tags = parseTags(value);
  const [draft, setDraft] = useState("");

  function add() {
    const t = draft.trim().replace(/;/g, "");
    if (!t) return;
    if (!tags.includes(t)) onChange(formatTags([...tags, t]));
    setDraft("");
  }

  return (
    <div>
      <div className="flex flex-wrap items-center gap-1.5">
        {tags.map((t) => (
          <span key={t} className="group inline-flex items-center gap-1">
            <TagChip tag={t} />
            <button
              type="button"
              aria-label={`Remove tag ${t}`}
              onClick={() => onChange(formatTags(tags.filter((x) => x !== t)))}
              className="rounded p-0.5 text-ink-faint hover:text-rose-300"
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        ))}
        {!tags.length && <span className="text-xs text-ink-faint">No tags</span>}
      </div>
      <div className="mt-2 flex gap-2">
        <Input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              add();
            }
          }}
          placeholder="Add a tag and press Enter"
          className="h-8 text-xs"
        />
        <Button type="button" size="sm" variant="outline" onClick={add}>
          <Plus className="h-3.5 w-3.5" />
          Add
        </Button>
      </div>
    </div>
  );
}
