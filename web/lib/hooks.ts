"use client";

import useSWR from "swr";
import { fetcher } from "./api";
import type { Organization, User } from "./types";

interface MeResponse {
  user: User;
  orgs: Organization[];
}

/** Current session (user + orgs). isUnauthorized is true on a 401. */
export function useMe() {
  const { data, error, isLoading, mutate } = useSWR<MeResponse>("/me", fetcher, {
    shouldRetryOnError: false,
  });
  return {
    user: data?.user,
    orgs: data?.orgs ?? [],
    isLoading,
    isUnauthorized: error?.status === 401,
    error,
    mutate,
  };
}

/** Poll a path on an interval (default 20s) — the lightweight, no-websocket
 *  refresh strategy used across the dashboard. */
export function usePolling<T>(path: string | null, intervalMs = 20000) {
  return useSWR<T>(path, fetcher, {
    refreshInterval: intervalMs,
    revalidateOnFocus: true,
    keepPreviousData: true,
  });
}
