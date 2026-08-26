// error.code drives behavior, error.message (already Indonesian from
// crm_be) is shown as-is — never re-translated client-side (TD phase 3
// §5: two sources of truth for one sentence eventually disagree).
import { ApiError } from "./api-types";

export type FieldErrors = Record<string, string>;

// validation_failed's details[] → field name -> message, for display
// under the field itself rather than as a global banner.
export function fieldErrorsFrom(err: unknown): FieldErrors {
  if (!(err instanceof ApiError) || err.code !== "validation_failed" || !err.details) {
    return {};
  }
  const out: FieldErrors = {};
  for (const detail of err.details) {
    out[detail.field] = fieldErrorMessage(detail.code);
  }
  return out;
}

function fieldErrorMessage(code: string): string {
  switch (code) {
    case "required":
      return "Wajib diisi.";
    case "invalid_json":
      return "Data tidak valid.";
    default:
      return "Nilai tidak valid.";
  }
}

export interface OrganizationOption {
  id: string;
  name: string;
}

// organization_selection_required (409, ADR-007) — a user with >1 active
// membership must pick one before login can complete.
export function organizationsFrom(err: unknown): OrganizationOption[] | null {
  if (err instanceof ApiError && err.code === "organization_selection_required") {
    const organizations = err.body.organizations;
    return Array.isArray(organizations) ? organizations : null;
  }
  return null;
}

// Fallback for every code without special UI treatment (invalid_credentials,
// email_not_verified, rate_limited, invalid_token, ...) — displayed as a
// plain banner, apa adanya.
export function globalMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Terjadi kesalahan. Coba lagi.";
}

// version_conflict (409, Aturan #35) — crm_be sends the row's CURRENT
// state in error.current so the screen can reload it without a second
// request. This is the ONE place in the product a save must never
// silently overwrite: the caller is required to show the conflict and
// let the user choose, never retry with the old version automatically.
export function versionConflictCurrent<T>(err: unknown): T | null {
  if (err instanceof ApiError && err.code === "version_conflict") {
    return (err.body.current as T) ?? null;
  }
  return null;
}

export function isLeadAlreadyConverted(err: unknown): boolean {
  return err instanceof ApiError && err.code === "lead_already_converted";
}
