export type Role = "owner" | "member";
export type Status = "up" | "down" | "unknown";

export interface User {
  id: string;
  username: string;
  email?: string;
  created_at: string;
}

export interface Organization {
  id: string;
  name: string;
  slug: string;
  role?: Role;
  created_at: string;
}

export interface Member {
  user_id: string;
  username: string;
  role: Role;
  joined_at: string;
}

export interface Monitor {
  id: string;
  organization_id: string;
  name: string;
  url: string;
  method: string;
  expected_status: number;
  interval_seconds: number;
  timeout_ms: number;
  follow_redirects: boolean;
  headers: Record<string, unknown>;
  fail_threshold: number;
  enabled: boolean;
  current_status: Status;
  consecutive_failures: number;
  last_checked_at: string | null;
  next_run_at: string;
  created_at: string;
}

export interface CheckResult {
  id: number;
  monitor_id: string;
  worker_id: string;
  checked_at: string;
  status: "up" | "down";
  status_code: number | null;
  response_time_ms: number | null;
  error: string | null;
}

export interface Incident {
  id: string;
  monitor_id: string;
  monitor_name: string;
  started_at: string;
  resolved_at: string | null;
  cause: string | null;
}

export interface MonthlySLA {
  month: string; // "2026-06"
  total: number;
  up: number;
  uptime_pct: number;
  avg_response_ms: number | null;
}

export interface DailySLA {
  day: string; // "2026-06-18"
  total: number;
  up: number;
  uptime_pct: number;
  avg_response_ms: number | null;
}

export interface Paginated<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface NotificationChannel {
  id: string;
  type: string;
  name: string;
  webhook_url: string;
  enabled: boolean;
  created_at: string;
}
