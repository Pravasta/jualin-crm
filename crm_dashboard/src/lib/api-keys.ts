// Typed wrapper for /v1/api-keys — shapes verified against
// crm_be/internal/apikey/handler_http.go's apiKeyJSON. Not paginated
// (returns a plain array via httpx.OK), same as /v1/memberships.
import { apiFetch } from "./api-client";

// "leads:write" is the only scope this codebase issues (ADR-004 aturan
// #4) — kept as a union of one rather than `string` so a typo in a
// future second scope shows up at compile time.
export type APIKeyScope = "leads:write";

export interface APIKey {
  id: string;
  key_prefix: string;
  name: string;
  scopes: APIKeyScope[];
  created_by_membership_id: string | null;
  created_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
  expires_at: string | null;
}

// secret does NOT exist on APIKey — only here, and only createAPIKey
// returns this type. Rendering a secret from the LIST is a type error,
// not just a logic bug that only shows up at runtime (Rule #21).
export interface CreatedAPIKey extends APIKey {
  secret: string;
}

export function listAPIKeys(signal?: AbortSignal): Promise<APIKey[]> {
  return apiFetch<APIKey[]>("/v1/api-keys", { signal });
}

export function createAPIKey(name: string): Promise<CreatedAPIKey> {
  return apiFetch<CreatedAPIKey>("/v1/api-keys", { method: "POST", body: { name } });
}

export function revokeAPIKey(id: string): Promise<void> {
  return apiFetch<void>(`/v1/api-keys/${id}`, { method: "DELETE" });
}
