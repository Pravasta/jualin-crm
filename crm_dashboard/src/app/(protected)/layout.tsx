import { SessionGate } from "@/lib/session-context";

// Route protection lives HERE, not in middleware.ts — middleware can't
// read the HttpOnly access token to verify it, so the only real check
// is asking the API (TD phase 3 §4.1). SessionGate calls GET /v1/me.
export default function ProtectedLayout({ children }: { children: React.ReactNode }) {
  return <SessionGate>{children}</SessionGate>;
}
