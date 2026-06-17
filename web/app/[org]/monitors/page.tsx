"use client";

import { use, useState } from "react";
import Link from "next/link";
import { ArrowRight, Plus, RefreshCw } from "lucide-react";
import { usePolling } from "@/lib/hooks";
import type { Monitor } from "@/lib/types";
import { timeAgo } from "@/lib/utils";
import { StatusPill } from "@/components/status";
import { MonitorForm } from "@/components/monitor-form";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";

export default function MonitorsPage({ params }: { params: Promise<{ org: string }> }) {
  const { org: slug } = use(params);
  const { data: monitors, isLoading, mutate } = usePolling<Monitor[]>(
    `/orgs/${slug}/monitors`,
    15000,
  );
  const [adding, setAdding] = useState(false);

  const up = monitors?.filter((m) => m.current_status === "up").length ?? 0;
  const down = monitors?.filter((m) => m.current_status === "down").length ?? 0;
  const pending = monitors?.filter((m) => m.current_status === "unknown").length ?? 0;

  return (
    <div className="animate-fade-up">
      <header className="mb-8 flex items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-3xl font-semibold">Monitors</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {monitors?.length ?? 0} domains watched
            {monitors && monitors.length > 0 && (
              <span className="ml-2 font-mono text-xs">
                <span className="text-up">{up} up</span> · <span className="text-down">{down} down</span> ·{" "}
                <span className="text-pending">{pending} pending</span>
              </span>
            )}
          </p>
        </div>
        <Button onClick={() => setAdding(true)}>
          <Plus className="h-4 w-4" /> Add monitor
        </Button>
      </header>

      {isLoading && !monitors ? (
        <SkeletonList />
      ) : monitors && monitors.length > 0 ? (
        <div className="space-y-2">
          {monitors.map((m) => (
            <Link key={m.id} href={`/${slug}/monitors/${m.id}`}>
              <Card className="group flex items-center gap-4 p-4 transition-all hover:border-primary/40 hover:shadow-md">
                <StatusPill status={m.current_status} live={m.current_status === "up"} />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{m.name}</p>
                  <p className="truncate text-sm text-muted-foreground">{m.url}</p>
                </div>
                <div className="hidden text-right sm:block">
                  <p className="text-xs text-muted-foreground">checked {timeAgo(m.last_checked_at)}</p>
                  <p className="font-mono text-xs text-muted-foreground">every {m.interval_seconds}s</p>
                </div>
                <ArrowRight className="h-4 w-4 text-muted-foreground transition-transform group-hover:translate-x-1" />
              </Card>
            </Link>
          ))}
        </div>
      ) : (
        <EmptyState onAdd={() => setAdding(true)} />
      )}

      <Dialog open={adding} onOpenChange={setAdding}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add monitor</DialogTitle>
            <DialogDescription>We&apos;ll start probing this endpoint right away.</DialogDescription>
          </DialogHeader>
          <MonitorForm
            orgSlug={slug}
            onSaved={() => { setAdding(false); mutate(); }}
            onCancel={() => setAdding(false)}
          />
        </DialogContent>
      </Dialog>
    </div>
  );
}

function SkeletonList() {
  return (
    <div className="space-y-2">
      {[0, 1, 2].map((i) => (
        <div key={i} className="h-16 animate-pulse rounded-lg border border-border bg-card" />
      ))}
    </div>
  );
}

function EmptyState({ onAdd }: { onAdd: () => void }) {
  return (
    <Card className="flex flex-col items-center justify-center gap-4 py-20 text-center">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
        <RefreshCw className="h-7 w-7" />
      </div>
      <div>
        <h3 className="font-serif text-xl font-semibold">No monitors yet</h3>
        <p className="mt-1 text-sm text-muted-foreground">Add your first domain to start tracking uptime.</p>
      </div>
      <Button onClick={onAdd}>
        <Plus className="h-4 w-4" /> Add monitor
      </Button>
    </Card>
  );
}
