"use client";

import { use } from "react";
import { CheckCircle2, AlertTriangle } from "lucide-react";
import { usePolling } from "@/lib/hooks";
import type { Incident } from "@/lib/types";
import { durationBetween, timeAgo } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";

export default function IncidentsPage({ params }: { params: Promise<{ org: string }> }) {
  const { org: slug } = use(params);
  const { data: incidents } = usePolling<Incident[]>(`/orgs/${slug}/incidents`, 20000);

  const ongoing = incidents?.filter((i) => !i.resolved_at).length ?? 0;

  return (
    <div className="animate-fade-up">
      <header className="mb-8">
        <h1 className="font-serif text-3xl font-semibold">Incidents</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {ongoing > 0 ? <span className="text-down">{ongoing} ongoing</span> : "All clear"} · {incidents?.length ?? 0} total
        </p>
      </header>

      {incidents && incidents.length > 0 ? (
        <div className="space-y-2">
          {incidents.map((i) => {
            const ongoing = !i.resolved_at;
            return (
              <Card key={i.id} className="p-4">
                <div className="flex items-start gap-4">
                  <div className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full ${ongoing ? "bg-down/10 text-down" : "bg-up/10 text-up"}`}>
                    {ongoing ? <AlertTriangle className="h-5 w-5" /> : <CheckCircle2 className="h-5 w-5" />}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="font-medium">{i.monitor_name}</p>
                      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${ongoing ? "bg-down/10 text-down" : "bg-up/10 text-up"}`}>
                        {ongoing ? "Ongoing" : "Resolved"}
                      </span>
                    </div>
                    {i.cause && <p className="mt-1 truncate text-sm text-muted-foreground">{i.cause}</p>}
                    <p className="mt-1 font-mono text-xs text-muted-foreground">
                      started {timeAgo(i.started_at)} · lasted {durationBetween(i.started_at, i.resolved_at)}
                    </p>
                  </div>
                </div>
              </Card>
            );
          })}
        </div>
      ) : (
        <Card className="flex flex-col items-center justify-center gap-3 py-20 text-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-up/10 text-up">
            <CheckCircle2 className="h-7 w-7" />
          </div>
          <h3 className="font-serif text-xl font-semibold">No incidents</h3>
          <p className="text-sm text-muted-foreground">Everything has been running smoothly.</p>
        </Card>
      )}
    </div>
  );
}
