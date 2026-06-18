import type { MonthlySLA } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

// monthLabel turns "2026-06" into "Jun 2026".
function monthLabel(month: string): string {
  const [y, m] = month.split("-").map(Number);
  if (!y || !m) return month;
  return new Date(y, m - 1, 1).toLocaleString(undefined, { month: "short", year: "numeric" });
}

// slaColor grades an uptime percentage (green ≥99.9, amber ≥99, else red).
function slaColor(pct: number): string {
  if (pct >= 99.9) return "text-up";
  if (pct >= 99) return "text-amber-500";
  return "text-down";
}

/** SlaCards renders one card per calendar month of count-based uptime. */
export function SlaCards({ sla }: { sla: MonthlySLA[] }) {
  if (sla.length === 0) {
    return (
      <Card>
        <CardContent className="p-6 text-sm text-muted-foreground">
          No checks recorded yet — SLA will appear once monitoring data accumulates.
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
      {sla.map((m) => (
        <Card key={m.month}>
          <CardContent className="p-5">
            <p className="text-xs uppercase tracking-wider text-muted-foreground">{monthLabel(m.month)}</p>
            <p className={cn("mt-1 font-serif text-3xl font-semibold", slaColor(m.uptime_pct))}>
              {m.uptime_pct.toFixed(2)}%
            </p>
            <p className="mt-2 text-xs text-muted-foreground">
              {m.up.toLocaleString()} / {m.total.toLocaleString()} checks up
              {m.avg_response_ms != null && <> · {m.avg_response_ms}ms avg</>}
            </p>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
