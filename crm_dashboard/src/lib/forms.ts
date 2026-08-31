// Typed wrapper for /v1/forms — shapes verified against
// crm_be/internal/form/handler_http.go's formJSON. Not paginated
// (returns a plain array via httpx.OK), same as /v1/api-keys and
// /v1/memberships.
//
// Forms are the third credential this product issues (public_key,
// ADR-005). Unlike api_keys there is NO secret half and NO optimistic
// locking: formJSON carries no `version`, and the backend's PATCH is
// last-write-wins — so nothing here round-trips a version the way
// leads/tasks forms must (Aturan #35 doesn't apply to forms).
import { apiFetch } from "./api-client";

// The six fixed field keys ADR-005 allows — there is no form builder, so
// this is a closed union, not `string`. Order here is the canonical
// render order the editor and the embed page both follow
// (crm_be/internal/form/entity.go's AllFieldKeys).
export const FIELD_KEYS = ["name", "email", "phone", "company", "message", "product"] as const;

export type FieldKey = (typeof FIELD_KEYS)[number];

export interface FieldConfig {
  enabled: boolean;
  required: boolean;
  label: string;
}

// A full record over every key — the backend always persists all six
// (form.DefaultFields seeds them on create), so a partial map would only
// invite "cfg is undefined" bugs in the editor.
export type Fields = Record<FieldKey, FieldConfig>;

export interface Form {
  id: string;
  public_key: string;
  name: string;
  fields: Fields;
  allowed_origins: string[];
  submit_count: number;
  created_by_membership_id: string | null;
  created_at: string;
  updated_at: string;
}

// Human-readable field names for the editor — what an Owner sees next to
// each toggle. NOT the customer-facing label (that's FieldConfig.label,
// which they edit); this just names which field the row configures.
export const FIELD_NAMES: Record<FieldKey, string> = {
  name: "Nama",
  email: "Email",
  phone: "Nomor WhatsApp",
  company: "Perusahaan",
  message: "Pesan",
  product: "Layanan diminati",
};

export interface UpdateFormInput {
  name?: string;
  fields?: Fields;
  allowed_origins?: string[];
}

// Mirrors crm_be/internal/form/entity.go's Fields.Validate — the backend
// rejects the same combinations with a generic validation_failed, so
// checking here is purely for an inline message the editor can show next
// to the offending row instead of a banner that doesn't say which field.
// Returns the first problem as { key, message }, or null when every
// field config is coherent.
export function firstFieldConfigError(fields: Fields): { key: FieldKey; message: string } | null {
  for (const key of FIELD_KEYS) {
    const cfg = fields[key];
    if (cfg.required && !cfg.enabled) {
      return { key, message: "Field wajib harus diaktifkan lebih dulu." };
    }
    if (cfg.enabled && cfg.label.trim() === "") {
      return { key, message: "Field yang aktif harus punya label." };
    }
  }
  return null;
}

export function listForms(signal?: AbortSignal): Promise<Form[]> {
  return apiFetch<Form[]>("/v1/forms", { signal });
}

export function getForm(id: string, signal?: AbortSignal): Promise<Form> {
  return apiFetch<Form>(`/v1/forms/${id}`, { signal });
}

// create takes only a name — the backend seeds every field enabled with
// a sensible Indonesian label (form.DefaultFields), and the Owner
// adjusts from the editor rather than from a blank, unusable form.
export function createForm(name: string): Promise<Form> {
  return apiFetch<Form>("/v1/forms", { method: "POST", body: { name } });
}

export function updateForm(id: string, input: UpdateFormInput): Promise<Form> {
  return apiFetch<Form>(`/v1/forms/${id}`, { method: "PATCH", body: input });
}

// Deactivating a form is a soft delete (deleted_at) — the public_key
// stops resolving, which is how ADR-005 says a form is revoked (there is
// no key rotation in Phase 6). NOT idempotent-success: a second call
// gets 404, same as customer delete.
export function deleteForm(id: string): Promise<void> {
  return apiFetch<void>(`/v1/forms/${id}`, { method: "DELETE" });
}
