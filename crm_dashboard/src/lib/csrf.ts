// csrf_token is deliberately NOT HttpOnly (crm_be/internal/shared/httpx/csrf.go) —
// this is what makes the double-submit mechanism work: a cross-site
// attacker can trigger a request with cookies attached, but can't READ
// this cookie to forge the header below.
const CSRF_COOKIE_NAME = "csrf_token";
export const CSRF_HEADER_NAME = "X-CSRF-Token";

export function readCsrfToken(): string | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(
    new RegExp(`(?:^|; )${CSRF_COOKIE_NAME}=([^;]*)`)
  );
  return match ? decodeURIComponent(match[1]) : null;
}
