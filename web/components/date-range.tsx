"use client";

import { Button } from "@/components/ui/button";

export interface Range {
  from: string; // YYYY-MM-DD
  to: string; // YYYY-MM-DD
}

// isoDay returns a YYYY-MM-DD string for `daysAgo` days before now (0 = today).
export function isoDay(daysAgo: number): string {
  const d = new Date();
  d.setDate(d.getDate() - daysAgo);
  return d.toISOString().slice(0, 10);
}

const PRESETS: { label: string; days: number }[] = [
  { label: "7d", days: 7 },
  { label: "30d", days: 30 },
  { label: "3mo", days: 90 },
];

/**
 * DateRange is a lightweight from/to picker (native date inputs) with quick
 * presets. History is capped at the last ~3 months, enforced via the min attr
 * and clamped again server-side.
 */
export function DateRange({ value, onChange }: { value: Range; onChange: (r: Range) => void }) {
  const min = isoDay(93);
  const max = isoDay(0);

  return (
    <div className="flex flex-wrap items-end gap-3">
      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        From
        <input
          type="date"
          value={value.from}
          min={min}
          max={value.to}
          onChange={(e) => onChange({ ...value, from: e.target.value })}
          className="h-8 rounded-md border border-border bg-transparent px-2 text-sm text-foreground"
        />
      </label>
      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        To
        <input
          type="date"
          value={value.to}
          min={value.from}
          max={max}
          onChange={(e) => onChange({ ...value, to: e.target.value })}
          className="h-8 rounded-md border border-border bg-transparent px-2 text-sm text-foreground"
        />
      </label>
      <div className="flex gap-1">
        {PRESETS.map((p) => (
          <Button
            key={p.label}
            variant="outline"
            size="sm"
            onClick={() => onChange({ from: isoDay(p.days), to: isoDay(0) })}
          >
            {p.label}
          </Button>
        ))}
      </div>
    </div>
  );
}
