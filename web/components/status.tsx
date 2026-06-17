import { cn } from "@/lib/utils";
import type { Status } from "@/lib/types";

const META: Record<Status, { label: string; dot: string; text: string; bg: string }> = {
  up: { label: "Operational", dot: "bg-up", text: "text-up", bg: "bg-up/10" },
  down: { label: "Down", dot: "bg-down", text: "text-down", bg: "bg-down/10" },
  unknown: { label: "Pending", dot: "bg-pending", text: "text-pending", bg: "bg-pending/10" },
};

/** A small pulsing status dot. `live` adds the expanding-ring animation. */
export function StatusDot({ status, live = false }: { status: Status; live?: boolean }) {
  return (
    <span className="relative inline-flex h-2.5 w-2.5">
      {live && (
        <span className={cn("absolute inline-flex h-full w-full rounded-full opacity-60 animate-ping-soft", META[status].dot)} />
      )}
      <span className={cn("relative inline-flex h-2.5 w-2.5 rounded-full", META[status].dot)} />
    </span>
  );
}

export function StatusPill({ status, live = false }: { status: Status; live?: boolean }) {
  const m = META[status];
  return (
    <span className={cn("inline-flex items-center gap-2 rounded-full px-2.5 py-1 text-xs font-medium", m.bg, m.text)}>
      <StatusDot status={status} live={live} />
      {m.label}
    </span>
  );
}
