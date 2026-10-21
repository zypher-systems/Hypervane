import { Field } from "@/components/ui/input";
import { STORAGE_CONTENT_TYPES, joinContent, parseContent } from "@/lib/storage";

export function ContentTypesField({
  value,
  onChange,
  allowed,
}: {
  value: string;
  onChange: (next: string) => void;
  allowed?: string[];
}) {
  const selected = new Set(parseContent(value));
  const types = STORAGE_CONTENT_TYPES.filter((t) => !allowed || allowed.includes(t.id));
  return (
    <Field label="Content">
      <div className="flex flex-wrap gap-x-4 gap-y-2">
        {types.map((t) => (
          <label key={t.id} className="flex items-center gap-2 text-sm text-ink">
            <input
              type="checkbox"
              checked={selected.has(t.id)}
              onChange={(e) => {
                const next = new Set(selected);
                if (e.target.checked) next.add(t.id);
                else next.delete(t.id);
                onChange(joinContent([...next]));
              }}
            />
            {t.label}
          </label>
        ))}
      </div>
      <p className="mt-1 text-xs text-ink-faint">What this store may hold. PBS is backups only; ZFS pools are disk and container volumes.</p>
    </Field>
  );
}
