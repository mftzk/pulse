import type { DailySLA } from "@/lib/types";
import { cn } from "@/lib/utils";

// dayColor grades a single day: green ≥99.9%, amber for partial downtime,
// red for heavy downtime, grey when there's no data or the day is in the future.
function dayColor(d: DailySLA | undefined, future: boolean): string {
  if (future || !d || d.total === 0) return "bg-muted";
  if (d.uptime_pct >= 99.9) return "bg-up";
  if (d.uptime_pct >= 90) return "bg-amber-400";
  return "bg-down";
}

function pad(n: number): string {
  return n < 10 ? `0${n}` : `${n}`;
}

const WEEKDAYS = ["S", "M", "T", "W", "T", "F", "S"];

/**
 * UptimeCalendar renders one calendar month as a 7-column grid (Sun–Sat),
 * coloring each day by its uptime, with the month's count-based SLA in the
 * header. `days` maps "YYYY-MM-DD" → that day's rollup; `todayISO` greys out
 * future days. `month` is 1-based.
 */
export function UptimeCalendar({
  year,
  month,
  days,
  todayISO,
}: {
  year: number;
  month: number;
  days: Map<string, DailySLA>;
  todayISO: string;
}) {
  const startDow = new Date(Date.UTC(year, month - 1, 1)).getUTCDay();
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate();
  const label = new Date(Date.UTC(year, month - 1, 1)).toLocaleString(undefined, {
    month: "long",
    year: "numeric",
  });

  // count-based monthly uptime, computed from the daily rollups we have
  let up = 0;
  let total = 0;
  for (let d = 1; d <= daysInMonth; d++) {
    const row = days.get(`${year}-${pad(month)}-${pad(d)}`);
    if (row) {
      up += row.up;
      total += row.total;
    }
  }
  const pct = total ? (up / total) * 100 : null;

  const cells: { key: string; iso?: string }[] = [];
  for (let i = 0; i < startDow; i++) cells.push({ key: `blank-${i}` });
  for (let d = 1; d <= daysInMonth; d++) {
    const iso = `${year}-${pad(month)}-${pad(d)}`;
    cells.push({ key: iso, iso });
  }

  return (
    <div className="min-w-[200px] flex-1">
      <div className="mb-3 flex items-baseline justify-between">
        <h3 className="text-sm font-semibold">{label}</h3>
        <span className="text-sm text-muted-foreground">{pct != null ? `${pct.toFixed(2)}%` : "—"}</span>
      </div>
      <div className="mb-1 grid grid-cols-7 gap-[3px] text-center text-[10px] text-muted-foreground">
        {WEEKDAYS.map((w, i) => (
          <span key={i}>{w}</span>
        ))}
      </div>
      <div className="grid grid-cols-7 gap-[3px]">
        {cells.map((c) => {
          if (!c.iso) return <span key={c.key} className="aspect-square" />;
          const d = days.get(c.iso);
          const future = c.iso > todayISO;
          return (
            <span
              key={c.key}
              title={
                future
                  ? `${c.iso} · upcoming`
                  : d && d.total > 0
                    ? `${c.iso} · ${d.uptime_pct.toFixed(2)}% up · ${d.up}/${d.total}`
                    : `${c.iso} · no data`
              }
              className={cn("aspect-square rounded-sm transition-transform hover:scale-110", dayColor(d, future))}
            />
          );
        })}
      </div>
    </div>
  );
}
