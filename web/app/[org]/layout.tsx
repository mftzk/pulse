"use client";

import { use, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useMe } from "@/lib/hooks";
import { Shell } from "@/components/shell";

export default function OrgLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ org: string }>;
}) {
  const { org: slug } = use(params);
  const router = useRouter();
  const { user, orgs, isLoading, isUnauthorized, mutate } = useMe();

  const current = orgs.find((o) => o.slug === slug);

  useEffect(() => {
    if (isLoading) return;
    if (isUnauthorized) {
      router.replace("/login");
    } else if (!current && orgs.length > 0) {
      // slug not one of my orgs -> go to my first org
      router.replace(`/${orgs[0].slug}/monitors`);
    }
  }, [isLoading, isUnauthorized, current, orgs, router]);

  if (isLoading || !user || !current) {
    return (
      <div className="flex min-h-screen items-center justify-center text-muted-foreground">
        <span className="animate-pulse font-serif text-lg">Pulse</span>
      </div>
    );
  }

  return (
    <Shell user={user} orgs={orgs} current={current} onOrgsChanged={() => mutate()}>
      {children}
    </Shell>
  );
}
