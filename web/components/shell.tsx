"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Activity, AlertTriangle, LogOut, Moon, Settings, Sun, Waypoints } from "lucide-react";
import type { Organization, User } from "@/lib/types";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { OrgSwitcher } from "@/components/org-switcher";
import { Button } from "@/components/ui/button";

function ThemeToggle() {
  const [dark, setDark] = useState(false);
  useEffect(() => {
    const stored = localStorage.getItem("pulse-theme");
    const isDark = stored === "dark";
    setDark(isDark);
    document.documentElement.classList.toggle("dark", isDark);
  }, []);
  function toggle() {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    localStorage.setItem("pulse-theme", next ? "dark" : "light");
  }
  return (
    <Button variant="ghost" size="icon" onClick={toggle} aria-label="Toggle theme">
      {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </Button>
  );
}

export function Shell({
  user,
  orgs,
  current,
  onOrgsChanged,
  children,
}: {
  user: User;
  orgs: Organization[];
  current: Organization;
  onOrgsChanged: () => void;
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const base = `/${current.slug}`;

  const nav = [
    { href: `${base}/monitors`, label: "Monitors", icon: Waypoints },
    { href: `${base}/incidents`, label: "Incidents", icon: AlertTriangle },
    { href: `${base}/settings`, label: "Settings", icon: Settings },
  ];

  async function logout() {
    await api.post("/auth/logout").catch(() => {});
    router.replace("/login");
  }

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 hidden h-screen w-64 shrink-0 flex-col gap-6 border-r border-border bg-sidebar p-4 md:flex">
        <Link href={`${base}/monitors`} className="flex items-center gap-2 px-2 pt-2">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Activity className="h-5 w-5" />
          </span>
          <span className="font-serif text-xl font-semibold tracking-tight">Pulse</span>
        </Link>

        <OrgSwitcher orgs={orgs} current={current} onChanged={onOrgsChanged} />

        <nav className="flex flex-1 flex-col gap-1">
          {nav.map((item) => {
            const active = pathname === item.href || pathname.startsWith(item.href + "/");
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                  active ? "bg-primary/10 text-primary" : "text-foreground/70 hover:bg-accent hover:text-foreground",
                )}
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div className="flex items-center justify-between border-t border-border pt-3">
          <div className="min-w-0">
            <p className="truncate text-sm font-medium">{user.username}</p>
            <button onClick={logout} className="flex items-center gap-1 text-xs text-muted-foreground hover:text-destructive">
              <LogOut className="h-3 w-3" /> Sign out
            </button>
          </div>
          <ThemeToggle />
        </div>
      </aside>

      <main className="flex-1 px-5 py-6 md:px-10 md:py-10">
        <div className="mx-auto max-w-5xl">{children}</div>
      </main>
    </div>
  );
}
