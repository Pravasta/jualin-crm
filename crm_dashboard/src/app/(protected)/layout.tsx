import { AppShell } from "@/components/app-shell";
import { SessionGate } from "@/lib/session-context";

// Route protection lives HERE, not in middleware.ts — middleware can't
// read the HttpOnly access token to verify it, so the only real check
// is asking the API (TD phase 3 §4.1). SessionGate calls GET /v1/me.
//
// AppShell sits inside SessionGate, not outside: it renders the
// organization name and the signed-in user, so it can only be built once
// the session actually resolved.
export default function ProtectedLayout({ children }: { children: React.ReactNode }) {
  return (
    <SessionGate>
      <AppShell>{children}</AppShell>
    </SessionGate>
  );
}
