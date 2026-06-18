"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Activity } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import type { Organization, User } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Turnstile, type TurnstileHandle } from "@/components/turnstile";

interface AuthResponse {
  user: User;
  orgs: Organization[];
}

interface AuthConfig {
  turnstile_site_key: string;
}

export function AuthForm({ mode }: { mode: "login" | "register" }) {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [siteKey, setSiteKey] = useState("");
  const [turnstileToken, setTurnstileToken] = useState("");
  const turnstileRef = useRef<TurnstileHandle>(null);

  const isLogin = mode === "login";
  const captchaRequired = !isLogin && siteKey !== "";

  // Register form fetches the Turnstile site key (public) so it can render the
  // captcha. When unset, the backend has captcha disabled and we skip it.
  useEffect(() => {
    if (isLogin) return;
    api
      .get<AuthConfig>("/auth/config")
      .then((cfg) => setSiteKey(cfg.turnstile_site_key || ""))
      .catch(() => setSiteKey(""));
  }, [isLogin]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (captchaRequired && !turnstileToken) {
      setError("Please complete the captcha.");
      return;
    }
    setLoading(true);
    try {
      const body = isLogin
        ? { username, password }
        : { username, email, password, turnstile_token: turnstileToken };
      const res = await api.post<AuthResponse>(`/auth/${isLogin ? "login" : "register"}`, body);
      const slug = res.orgs?.[0]?.slug;
      router.replace(slug ? `/${slug}/monitors` : "/");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong");
      setLoading(false);
      // Turnstile tokens are single-use; reset so the user can retry.
      if (captchaRequired) {
        setTurnstileToken("");
        turnstileRef.current?.reset();
      }
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-sm animate-fade-up">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-sm">
            <Activity className="h-6 w-6" />
          </div>
          <h1 className="font-serif text-3xl font-semibold">
            {isLogin ? "Welcome back" : "Create your account"}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {isLogin ? "Sign in to keep watch over your endpoints." : "Start monitoring your domains in minutes."}
          </p>
        </div>

        <form onSubmit={onSubmit} className="space-y-4 rounded-xl border border-border bg-card p-6 shadow-sm">
          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
              minLength={3}
            />
          </div>
          {!isLogin && (
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete="email"
                required
              />
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={isLogin ? "current-password" : "new-password"}
              required
              minLength={6}
            />
          </div>
          {captchaRequired && (
            <Turnstile ref={turnstileRef} siteKey={siteKey} onToken={setTurnstileToken} />
          )}
          {error && <p className="text-sm text-destructive">{error}</p>}
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? "Please wait…" : isLogin ? "Sign in" : "Create account"}
          </Button>
        </form>

        <p className="mt-6 text-center text-sm text-muted-foreground">
          {isLogin ? "No account yet? " : "Already have an account? "}
          <Link href={isLogin ? "/register" : "/login"} className="font-medium text-primary hover:underline">
            {isLogin ? "Create one" : "Sign in"}
          </Link>
        </p>
      </div>
    </div>
  );
}
