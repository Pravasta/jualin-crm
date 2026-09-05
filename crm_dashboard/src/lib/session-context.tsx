"use client";

// Session is enforced by calling GET /v1/me from the protected layout —
// never by reading the access token, which is HttpOnly and can't be
// read from JavaScript at all (TD phase 3 §4.1).
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { me, type MeResponse } from "./auth";

interface SessionContextValue {
  session: MeResponse;
  // refresh re-fetches GET /v1/me and replaces the cached session — #125's
  // ONLY caller: after a test checkout changes plan.code, the Langganan
  // screen needs the new plan reflected immediately, not after the next
  // full navigation (SessionGate itself only fetches once, on mount).
  refresh: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | null>(null);

// Only valid inside (protected)/layout.tsx's SessionGate — every screen
// under it is guaranteed a resolved session by the time it renders.
export function useSession(): MeResponse {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error("useSession must be used within the protected layout");
  }
  return ctx.session;
}

export function useSessionRefresh(): () => Promise<void> {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error("useSessionRefresh must be used within the protected layout");
  }
  return ctx.refresh;
}

export function SessionGate({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [session, setSession] = useState<MeResponse | null>(null);

  useEffect(() => {
    let cancelled = false;
    me()
      .then((result) => {
        if (!cancelled) setSession(result);
      })
      .catch(() => {
        // apiFetch already redirects to /login on a 401 that survives
        // refresh (api-client.ts) — this covers every other failure
        // (network error, unexpected 5xx) the same way: no dashboard
        // screen renders without a confirmed session.
        if (!cancelled) router.replace("/login");
      });
    return () => {
      cancelled = true;
    };
  }, [router]);

  const refresh = useCallback(async () => {
    const result = await me();
    setSession(result);
  }, []);

  if (!session) {
    return null;
  }

  return <SessionContext.Provider value={{ session, refresh }}>{children}</SessionContext.Provider>;
}
