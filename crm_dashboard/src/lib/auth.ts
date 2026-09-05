// Typed wrappers for /v1/auth/* and /v1/me — shapes verified directly
// against crm_be/internal/auth/{handler_http,entity}.go, not guessed.
import { apiFetch } from "./api-client";

// dashboard always sends client: "dashboard" (crm_be's ClientDashboard) —
// this is what makes the handler respond with Set-Cookie instead of
// tokens in the body (TD phase 1 §5 acceptance criteria).
const CLIENT = "dashboard";

export interface RegisterInput {
  organizationName: string;
  fullName: string;
  email: string;
  password: string;
}

export interface RegisterResult {
  user_id: string;
  organization_id: string;
}

export function register(input: RegisterInput): Promise<RegisterResult> {
  return apiFetch<RegisterResult>("/v1/auth/register", {
    method: "POST",
    body: {
      organization_name: input.organizationName,
      full_name: input.fullName,
      email: input.email,
      password: input.password,
    },
  });
}

export function verifyEmail(token: string): Promise<{ status: string }> {
  return apiFetch("/v1/auth/verify-email", { method: "POST", body: { token } });
}

export function resendVerification(email: string): Promise<void> {
  return apiFetch("/v1/auth/verify-email/resend", { method: "POST", body: { email } });
}

export interface LoginInput {
  email: string;
  password: string;
  organizationId?: string;
}

export function login(input: LoginInput): Promise<{ status: string }> {
  return apiFetch("/v1/auth/login", {
    method: "POST",
    body: {
      email: input.email,
      password: input.password,
      client: CLIENT,
      organization_id: input.organizationId,
    },
  });
}

export function logout(): Promise<void> {
  return apiFetch("/v1/auth/logout", { method: "POST" });
}

export function forgotPassword(email: string): Promise<void> {
  return apiFetch("/v1/auth/password/forgot", { method: "POST", body: { email } });
}

export function resetPassword(token: string, password: string): Promise<{ status: string }> {
  return apiFetch("/v1/auth/password/reset", { method: "POST", body: { token, password } });
}

// plan.channels is keyed by Record<string, boolean>, NOT Record<PlanChannel,
// boolean> — this is the wire shape (subscription TD §4), and a key this
// dashboard doesn't recognize must still be a valid value, not a type
// error. lib/plan.ts's isChannelOpen is the only place that should read
// this field; it treats a missing key as closed (fail closed, mirroring
// crm_be's channelsFor).
export interface MeResponse {
  user_id: string;
  email: string;
  full_name: string;
  organization_id: string;
  organization_name: string;
  membership_id: string;
  role: "owner" | "admin" | "manager" | "employee";
  plan: {
    code: string;
    channels: Record<string, boolean>;
    // limits/usage added #125 (subscription TD §7) — 0 in EITHER means
    // "tanpa batas", never "none" (lib/plan.ts's formatLimit/formatUsage
    // are the only place that reads what 0 means, mirroring crm_be's
    // allows()).
    limits: {
      leads_per_month: number;
      seats: number;
    };
    usage: {
      leads_this_month: number;
      seats_used: number;
    };
    // One source of truth for "does this deployment allow the test
    // upgrade button" (TD 8.5 §7) — never a frontend-only flag.
    test_checkout_available: boolean;
  };
}

export function me(): Promise<MeResponse> {
  return apiFetch<MeResponse>("/v1/me");
}
