import type { DailySLA } from "@/lib/types";
import { cn } from "@/lib/utils";

// barColor grades a day's uptime (green ≥99.9, amber ≥95, else red).
function barColor(pct: number): string {
  if (pct >= 99.9) return "bg-up";
  if (pct >= 95) return "bg-amber-500";
  return "bg-down";
}

/**
 * DailyBars renders per-day uptime as a bar chart (height ∝ uptime%) plus a
 * range summary. Cheap, no charting library — mirrors components/heartbeat.tsx.
 */
export function DailyBars({ daily }: { daily: DailySLA[] }) {
  if (daily.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">No checks in this range.</p>;
  }

  const totals = daily.reduce(
    (a, d) => ({ up: a.up + d.up, total: a.total + d.total }),
    { up: 0, total: 0 },
  );
  const rangeUptime = totals.total ? (totals.up / totals.total) * 100 : 0;
  const totalDown = totals.total - totals.up;

  return (
    <div>
      <div className="mb-4 flex flex-wrap gap-6 text-sm">
        <div>
          <span className="text-muted-foreground">Range uptime: </span>
          <span className="font-medium">{rangeUptime.toFixed(2)}%</span>
        </div>
        <div>
          <span className="text-muted-foreground">Down checks: </span>
          <span className="font-medium">{totalDown.toLocaleString()}</span>
        </div>
        <div>
          <span className="text-muted-foreground">Total checks: </span>
          <span className="font-medium">{totals.total.toLocaleString()}</span>
        </div>
      </div>

      <div className="flex h-24 items-end gap-[2px]" aria-label="daily uptime history">
        {daily.map((d) => (
          <span
            key={d.day}
            title={`${d.day} · ${d.uptime_pct.toFixed(2)}% up · ${d.up}/${d.total}${
              d.avg_response_ms != null ? ` · ${d.avg_response_ms}ms avg` : ""
            }`}
            className={cn("min-w-[3px] flex-1 rounded-sm transition-transform hover:opacity-80", barColor(d.uptime_pct))}
            style={{ height: `${Math.max(4, d.uptime_pct)}%` }}
          />
        ))}
      </div>
    </div>
  );
}
