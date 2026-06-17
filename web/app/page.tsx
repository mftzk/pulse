"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useMe } from "@/lib/hooks";

export default function Home() {
  const router = useRouter();
  const { orgs, isLoading, isUnauthorized } = useMe();

  useEffect(() => {
    if (isLoading) return;
    if (isUnauthorized || orgs.length === 0) {
      router.replace("/login");
    } else {
      router.replace(`/${orgs[0].slug}/monitors`);
    }
  }, [isLoading, isUnauthorized, orgs, router]);

  return (
    <div className="flex min-h-screen items-center justify-center text-muted-foreground">
      <span className="animate-pulse font-serif text-lg">Pulse</span>
    </div>
  );
}
