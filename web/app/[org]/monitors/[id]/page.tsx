"use client";

import { use, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, ExternalLink, Pencil, Trash2 } from "lucide-react";
import { usePolling } from "@/lib/hooks";
import { api } from "@/lib/api";
import type { CheckResult, DailySLA, Monitor, MonthlySLA, Paginated } from "@/lib/types";
import { timeAgo } from "@/lib/utils";
import { StatusPill } from "@/components/status";
import { Heartbeat } from "@/components/heartbeat";
import { MonitorForm } from "@/components/monitor-form";
import { SlaCards } from "@/components/sla-cards";
import { DateRange, isoDay, type Range } from "@/components/date-range";
import { DailyBars } from "@/components/daily-bars";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";

const PAGE_SIZE = 50;

// nextDay returns the day after an ISO date, used as the exclusive upper bound
// so a selected end date is included in the range.
function nextDay(iso: string): string {
  const d = new Date(iso + "T00:00:00Z");
  d.setUTCDate(d.getUTCDate() + 1);
  return d.toISOString().slice(0, 10);
}

export default function MonitorDetailPage({
  params,
}: {
  params: Promise<{ org: string; id: string }>;
}) {
  const { org: slug, id } = use(params);
  const router = useRouter();
  const base = `/orgs/${slug}/monitors/${id}`;

  const { data: monitor, mutate } = usePolling<Monitor>(base, 15000);
  const { data: results } = usePolling<CheckResult[]>(`${base}/results?limit=50`, 15000);
  const { data: sla } = usePolling<MonthlySLA[]>(`${base}/sla?months=3`, 60000);

  const [range, setRange] = useState<Range>({ from: isoDay(30), to: isoDay(0) });
  const [offset, setOffset] = useState(0);
  const rangeQuery = `from=${range.from}&to=${nextDay(range.to)}`;
  const { data: daily } = usePolling<DailySLA[]>(`${base}/results/daily?${rangeQuery}`, 60000);
  const { data: history } = usePolling<Paginated<CheckResult>>(
    `${base}/results/range?${rangeQuery}&limit=${PAGE_SIZE}&offset=${offset}`,
    30000,
  );

  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);

  if (!monitor) {
    return <div className="h-40 animate-pulse rounded-lg border border-border bg-card" />;
  }

  const withTimes = (results ?? []).filter((r) => r.response_time_ms != null);
  const avg = withTimes.length
    ? Math.round(withTimes.reduce((a, r) => a + (r.response_time_ms ?? 0), 0) / withTimes.length)
    : null;
  const thisMonth = sla?.[0]?.uptime_pct;

  function changeRange(r: Range) {
    setOffset(0); // reset pagination when the window changes
    setRange(r);
  }

  async function remove() {
    await api.del(base);
    router.replace(`/${slug}/monitors`);
  }

  return (
    <div className="animate-fade-up">
      <Link href={`/${slug}/monitors`} className="mb-6 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="h-4 w-4" /> All monitors
      </Link>

      <header className="mb-8 flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-3">
            <h1 className="font-serif text-3xl font-semibold">{monitor.name}</h1>
            <StatusPill status={monitor.current_status} live={monitor.current_status === "up"} />
          </div>
          <a href={monitor.url} target="_blank" rel="noreferrer" className="mt-1 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-primary">
            {monitor.url} <ExternalLink className="h-3 w-3" />
          </a>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => setEditing(true)}><Pencil className="h-4 w-4" /> Edit</Button>
          <Button variant="outline" size="sm" className="text-destructive hover:bg-destructive/10" onClick={() => setDeleting(true)}>
            <Trash2 className="h-4 w-4" /> Delete
          </Button>
        </div>
      </header>

      <div className="mb-6 grid grid-cols-2 gap-4 md:grid-cols-4">
        <Stat label="Status" value={monitor.current_status} accent />
        <Stat label="Uptime (this month)" value={thisMonth != null ? `${thisMonth.toFixed(2)}%` : "—"} />
        <Stat label="Avg response" value={avg != null ? `${avg} ms` : "—"} />
        <Stat label="Last checked" value={timeAgo(monitor.last_checked_at)} />
      </div>

      <section className="mb-8">
        <p className="mb-3 text-sm font-medium text-muted-foreground">Monthly SLA</p>
        <SlaCards sla={sla ?? []} />
      </section>

      <Card className="mb-6">
        <CardContent className="p-6">
          <p className="mb-4 text-sm font-medium text-muted-foreground">Recent checks</p>
          <Heartbeat results={results ?? []} />
        </CardContent>
      </Card>

      <section className="mb-6">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-4">
          <p className="text-sm font-medium text-muted-foreground">History</p>
          <DateRange value={range} onChange={changeRange} />
        </div>
        <Card className="mb-6">
          <CardContent className="p-6">
            <DailyBars daily={daily ?? []} />
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-0">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs uppercase tracking-wider text-muted-foreground">
                  <th className="px-6 py-3 font-medium">Time</th>
                  <th className="px-6 py-3 font-medium">Status</th>
                  <th className="px-6 py-3 font-medium">Code</th>
                  <th className="px-6 py-3 font-medium">Response</th>
                  <th className="px-6 py-3 font-medium">Detail</th>
                </tr>
              </thead>
              <tbody>
                {(history?.data ?? []).map((r) => (
                  <tr key={r.id} className="border-b border-border/60 last:border-0">
                    <td className="px-6 py-3 text-muted-foreground">{new Date(r.checked_at).toLocaleString()}</td>
                    <td className="px-6 py-3">
                      <span className={r.status === "up" ? "text-up" : "text-down"}>{r.status}</span>
                    </td>
                    <td className="px-6 py-3 font-mono text-xs">{r.status_code ?? "—"}</td>
                    <td className="px-6 py-3 font-mono text-xs">{r.response_time_ms != null ? `${r.response_time_ms}ms` : "—"}</td>
                    <td className="max-w-[14rem] truncate px-6 py-3 text-xs text-muted-foreground">{r.error ?? ""}</td>
                  </tr>
                ))}
                {history && history.data.length === 0 && (
                  <tr><td colSpan={5} className="px-6 py-8 text-center text-muted-foreground">No checks in this range.</td></tr>
                )}
              </tbody>
            </table>
          </CardContent>
        </Card>

        {history && history.total > 0 && (
          <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
            <span>
              {history.offset + 1}–{Math.min(history.offset + PAGE_SIZE, history.total)} of {history.total.toLocaleString()}
            </span>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
                Previous
              </Button>
              <Button variant="outline" size="sm" disabled={offset + PAGE_SIZE >= history.total} onClick={() => setOffset(offset + PAGE_SIZE)}>
                Next
              </Button>
            </div>
          </div>
        )}
      </section>

      <Dialog open={editing} onOpenChange={setEditing}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit monitor</DialogTitle>
            <DialogDescription>Changes take effect on the next check.</DialogDescription>
          </DialogHeader>
          <MonitorForm
            orgSlug={slug}
            monitor={monitor}
            onSaved={() => { setEditing(false); mutate(); }}
            onCancel={() => setEditing(false)}
          />
        </DialogContent>
      </Dialog>

      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete monitor?</DialogTitle>
            <DialogDescription>
              This permanently removes <strong>{monitor.name}</strong> and its check history.
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setDeleting(false)}>Cancel</Button>
            <Button variant="destructive" onClick={remove}>Delete</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function Stat({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-xs uppercase tracking-wider text-muted-foreground">{label}</p>
        <p className={`mt-1 font-serif text-2xl font-semibold ${accent ? "capitalize text-primary" : ""}`}>{value}</p>
      </CardContent>
    </Card>
  );
}
