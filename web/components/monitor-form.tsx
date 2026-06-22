"use client";

import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { Monitor } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface Props {
  orgSlug: string;
  monitor?: Monitor;
  onSaved: (m: Monitor) => void;
  onCancel: () => void;
}

const INTERVALS = [
  { label: "30 seconds", value: 30 },
  { label: "1 minute", value: 60 },
  { label: "5 minutes", value: 300 },
  { label: "15 minutes", value: 900 },
];

// How often to repeat the alert while a monitor stays down. 0 = off.
const REMINDERS = [
  { label: "Off", value: 0 },
  { label: "Every 5 minutes", value: 300 },
  { label: "Every 10 minutes", value: 600 },
  { label: "Every 30 minutes", value: 1800 },
  { label: "Every hour", value: 3600 },
];

const STATUS_CLASSES = [
  { label: "2xx · Success", value: 2 },
  { label: "3xx · Redirect", value: 3 },
  { label: "4xx · Client error", value: 4 },
  { label: "5xx · Server error", value: 5 },
];

// Normalize a stored expected_status into a class digit (2..5).
// Legacy values: 0 (any 2xx) -> 2; an exact code like 404 -> its class (4).
function toStatusClass(v: number): number {
  if (v >= 100) return Math.floor(v / 100);
  if (v >= 2 && v <= 5) return v;
  return 2;
}

export function MonitorForm({ orgSlug, monitor, onSaved, onCancel }: Props) {
  const [name, setName] = useState(monitor?.name ?? "");
  const [url, setUrl] = useState(monitor?.url ?? "https://");
  const [method, setMethod] = useState(monitor?.method ?? "GET");
  const [interval, setInterval] = useState(monitor?.interval_seconds ?? 60);
  const [timeout, setTimeout] = useState(monitor?.timeout_ms ?? 10000);
  // expected_status is stored as a status CLASS digit (2=2xx, 3=3xx, 4=4xx, 5=5xx).
  // Map any legacy value (0=any 2xx, or an exact code like 404) to its class.
  const [expected, setExpected] = useState(toStatusClass(monitor?.expected_status ?? 0));
  const [failThreshold, setFailThreshold] = useState(monitor?.fail_threshold ?? 1);
  const [reminder, setReminder] = useState(monitor?.reminder_interval_seconds ?? 600);
  const [followRedirects, setFollowRedirects] = useState(monitor?.follow_redirects ?? true);
  const [enabled, setEnabled] = useState(monitor?.enabled ?? true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const payload = {
      name,
      url,
      method,
      interval_seconds: interval,
      timeout_ms: timeout,
      expected_status: expected,
      fail_threshold: failThreshold,
      reminder_interval_seconds: reminder,
      follow_redirects: followRedirects,
      enabled,
    };
    try {
      const saved = monitor
        ? await api.put<Monitor>(`/orgs/${orgSlug}/monitors/${monitor.id}`, payload)
        : await api.post<Monitor>(`/orgs/${orgSlug}/monitors`, payload);
      onSaved(saved);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save");
      setBusy(false);
    }
  }

  const selectCls =
    "h-9 w-full rounded-md border border-input bg-card px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

  return (
    <form onSubmit={submit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="m-name">Name</Label>
        <Input id="m-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="My API" required autoFocus />
      </div>
      <div className="space-y-2">
        <Label htmlFor="m-url">URL</Label>
        <Input id="m-url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/health" required />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label htmlFor="m-method">Method</Label>
          <select id="m-method" className={selectCls} value={method} onChange={(e) => setMethod(e.target.value)}>
            {["GET", "HEAD", "POST"].map((m) => <option key={m}>{m}</option>)}
          </select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="m-interval">Check every</Label>
          <select id="m-interval" className={selectCls} value={interval} onChange={(e) => setInterval(Number(e.target.value))}>
            {INTERVALS.map((i) => <option key={i.value} value={i.value}>{i.label}</option>)}
          </select>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div className="space-y-2">
          <Label htmlFor="m-timeout">Timeout (ms)</Label>
          <Input id="m-timeout" type="number" min={500} value={timeout} onChange={(e) => setTimeout(Number(e.target.value))} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="m-expected">Expected status</Label>
          <select id="m-expected" className={selectCls} value={expected} onChange={(e) => setExpected(Number(e.target.value))}>
            {STATUS_CLASSES.map((c) => <option key={c.value} value={c.value}>{c.label}</option>)}
          </select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="m-threshold">Fail threshold</Label>
          <Input id="m-threshold" type="number" min={1} value={failThreshold} onChange={(e) => setFailThreshold(Number(e.target.value))} />
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="m-reminder">Repeat alert while down</Label>
        <select id="m-reminder" className={selectCls} value={reminder} onChange={(e) => setReminder(Number(e.target.value))}>
          {REMINDERS.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
        </select>
        <p className="text-xs text-muted-foreground">Re-send the down notification on this interval until the monitor recovers.</p>
      </div>

      <div className="flex items-center gap-6 pt-1">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" className="h-4 w-4 accent-primary" checked={followRedirects} onChange={(e) => setFollowRedirects(e.target.checked)} />
          Follow redirects
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" className="h-4 w-4 accent-primary" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Enabled
        </label>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}
      <div className="flex justify-end gap-2 pt-2">
        <Button type="button" variant="outline" onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={busy}>{monitor ? "Save changes" : "Create monitor"}</Button>
      </div>
    </form>
  );
}
