"use client";

import { use, useState } from "react";
import useSWR from "swr";
import { Trash2, UserPlus, Webhook } from "lucide-react";
import { api, ApiError, fetcher } from "@/lib/api";
import { useMe } from "@/lib/hooks";
import type { Member, NotificationChannel } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function SettingsPage({ params }: { params: Promise<{ org: string }> }) {
  const { org: slug } = use(params);
  const { orgs, user } = useMe();
  const isOwner = orgs.find((o) => o.slug === slug)?.role === "owner";

  return (
    <div className="animate-fade-up space-y-8">
      <header>
        <h1 className="font-serif text-3xl font-semibold">Settings</h1>
        <p className="mt-1 text-sm text-muted-foreground">Manage members and alerting for this organization.</p>
      </header>
      <MembersSection slug={slug} isOwner={isOwner} currentUserId={user?.id} />
      <ChannelsSection slug={slug} />
    </div>
  );
}

function MembersSection({ slug, isOwner, currentUserId }: { slug: string; isOwner: boolean; currentUserId?: string }) {
  const { data: members, mutate } = useSWR<Member[]>(`/orgs/${slug}/members`, fetcher);
  const [username, setUsername] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function add(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.post(`/orgs/${slug}/members`, { username });
      setUsername("");
      mutate();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to add member");
    } finally {
      setBusy(false);
    }
  }

  async function remove(userId: string) {
    await api.del(`/orgs/${slug}/members/${userId}`);
    mutate();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="font-serif">Members</CardTitle>
        <CardDescription>Users in this organization. A user can belong to many organizations.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="divide-y divide-border rounded-lg border border-border">
          {(members ?? []).map((m) => (
            <div key={m.user_id} className="flex items-center gap-3 px-4 py-3">
              <span className="flex h-8 w-8 items-center justify-center rounded-full bg-muted font-mono text-xs font-semibold">
                {m.username.slice(0, 2).toUpperCase()}
              </span>
              <span className="flex-1 text-sm font-medium">{m.username}</span>
              <span className="rounded-full bg-secondary px-2 py-0.5 text-xs text-secondary-foreground">{m.role}</span>
              {(isOwner || m.user_id === currentUserId) && (
                <Button variant="ghost" size="icon" className="text-muted-foreground hover:text-destructive" onClick={() => remove(m.user_id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              )}
            </div>
          ))}
        </div>

        {isOwner && (
          <form onSubmit={add} className="flex items-end gap-2">
            <div className="flex-1 space-y-2">
              <Label htmlFor="member-username">Add member by username</Label>
              <Input id="member-username" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="existing username" required />
            </div>
            <Button type="submit" disabled={busy || !username.trim()}><UserPlus className="h-4 w-4" /> Add</Button>
          </form>
        )}
        {error && <p className="text-sm text-destructive">{error}</p>}
      </CardContent>
    </Card>
  );
}

function ChannelsSection({ slug }: { slug: string }) {
  const { data: channels, mutate } = useSWR<NotificationChannel[]>(`/orgs/${slug}/channels`, fetcher);
  const [name, setName] = useState("");
  const [webhook, setWebhook] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function add(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.post(`/orgs/${slug}/channels`, { name: name || "Discord", webhook_url: webhook });
      setName("");
      setWebhook("");
      mutate();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to add channel");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    await api.del(`/orgs/${slug}/channels/${id}`);
    mutate();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="font-serif">Discord alerts</CardTitle>
        <CardDescription>We post here when a monitor goes down or recovers.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {channels && channels.length > 0 && (
          <div className="divide-y divide-border rounded-lg border border-border">
            {channels.map((c) => (
              <div key={c.id} className="flex items-center gap-3 px-4 py-3">
                <Webhook className="h-4 w-4 text-primary" />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{c.name}</p>
                  <p className="truncate font-mono text-xs text-muted-foreground">{c.webhook_url}</p>
                </div>
                <Button variant="ghost" size="icon" className="text-muted-foreground hover:text-destructive" onClick={() => remove(c.id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}
          </div>
        )}

        <form onSubmit={add} className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-[1fr_2fr]">
            <div className="space-y-2">
              <Label htmlFor="ch-name">Name</Label>
              <Input id="ch-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Discord" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ch-url">Webhook URL</Label>
              <Input id="ch-url" value={webhook} onChange={(e) => setWebhook(e.target.value)} placeholder="https://discord.com/api/webhooks/..." required />
            </div>
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <Button type="submit" disabled={busy || !webhook.trim()}><Webhook className="h-4 w-4" /> Add webhook</Button>
        </form>
      </CardContent>
    </Card>
  );
}
