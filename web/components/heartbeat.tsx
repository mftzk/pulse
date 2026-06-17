import { cn } from "@/lib/utils";
import type { CheckResult } from "@/lib/types";

/**
 * Heartbeat bar — renders recent check results as colored ticks (newest right),
 * the way uptime dashboards visualize history. Cheap: no charting library.
 */
export function Heartbeat({ results, slots = 50 }: { results: CheckResult[]; slots?: number }) {
  // results come newest-first; reverse to oldest->newest, then pad on the left
  const ticks = [...results].slice(0, slots).reverse();
  const pad = Math.max(0, slots - ticks.length);

  return (
    <div className="flex items-end gap-[3px]" aria-label="recent check history">
      {Array.from({ length: pad }).map((_, i) => (
        <span key={`pad-${i}`} className="h-7 w-[6px] rounded-full bg-muted" />
      ))}
      {ticks.map((r) => (
        <span
          key={r.id}
          title={`${new Date(r.checked_at).toLocaleString()} · ${r.status}${
            r.response_time_ms != null ? ` · ${r.response_time_ms}ms` : ""
          }${r.error ? ` · ${r.error}` : ""}`}
          className={cn(
            "h-7 w-[6px] rounded-full transition-transform hover:scale-y-110",
            r.status === "up" ? "bg-up" : "bg-down",
          )}
        />
      ))}
    </div>
  );
}
