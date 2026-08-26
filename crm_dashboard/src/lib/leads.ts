// Typed wrapper for /v1/leads — shapes verified against
// crm_be/internal/lead/{handler_http,entity}.go's leadJSON, not guessed.
import { apiFetch, apiFetchList } from "./api-client";
import type { Meta } from "./api-types";
import type { LeadSource, LeadStatus, LostReason } from "./labels";

export interface Lead {
  id: string;
  lead_number: number;
  name: string;
  email: string | null;
  phone: string | null;
  phone_e164: string | null;
  company: string | null;
  notes: string | null;
  status: LeadStatus;
  lost_reason: LostReason | null;
  source: LeadSource;
  assigned_to_membership_id: string | null;
  version: number;
  created_by_membership_id: string | null;
  created_at: string;
  updated_at: string;
}

// undefined fields are simply omitted from the query string — a filter
// the user hasn't touched shouldn't send an empty/misleading param.
export interface ListLeadsFilter {
  status?: LeadStatus[];
  source?: LeadSource[];
  /** A membership id, or the literal "none" for "tanpa pemilik aktif". */
  assignedTo?: string;
  q?: string;
  /** ISO 8601 UTC, e.g. from dateInputToUTCRange in @/lib/date. */
  createdFrom?: string;
  createdTo?: string;
  page?: number;
  perPage?: number;
}

export function buildQuery(filter: ListLeadsFilter): string {
  const params = new URLSearchParams();
  if (filter.status?.length) params.set("status", filter.status.join(","));
  if (filter.source?.length) params.set("source", filter.source.join(","));
  if (filter.assignedTo) params.set("assigned_to", filter.assignedTo);
  if (filter.q) params.set("q", filter.q);
  if (filter.createdFrom) params.set("created_from", filter.createdFrom);
  if (filter.createdTo) params.set("created_to", filter.createdTo);
  if (filter.page) params.set("page", String(filter.page));
  if (filter.perPage) params.set("per_page", String(filter.perPage));
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

export function listLeads(
  filter: ListLeadsFilter,
  signal?: AbortSignal
): Promise<{ data: Lead[]; meta: Meta }> {
  return apiFetchList<Lead>(`/v1/leads${buildQuery(filter)}`, { signal });
}

export interface CreateLeadInput {
  name: string;
  email?: string;
  phone?: string;
}

// source is always "manual" — the only source a human typing into this
// screen can produce (glossary.md: source is the CAPTURE method, and a
// dashboard form is exactly that). api/form/webhook leads are never
// created here.
export function createLead(input: CreateLeadInput): Promise<Lead> {
  return apiFetch<Lead>("/v1/leads", {
    method: "POST",
    body: {
      name: input.name,
      email: input.email || undefined,
      phone: input.phone || undefined,
      source: "manual",
    },
  });
}

export function getLead(id: string, signal?: AbortSignal): Promise<Lead> {
  return apiFetch<Lead>(`/v1/leads/${id}`, { signal });
}

// Every write below requires `version` — the value read from the Lead
// currently on screen, sent back unchanged (TD §6). A form that forgets
// it works exactly once, then fails 409 on every subsequent save.

export interface UpdateLeadInput {
  version: number;
  name?: string;
  email?: string | null;
  phone?: string | null;
  company?: string | null;
  notes?: string | null;
}

export function updateLead(id: string, input: UpdateLeadInput): Promise<Lead> {
  return apiFetch<Lead>(`/v1/leads/${id}`, { method: "PATCH", body: input });
}

export function updateLeadStatus(
  id: string,
  input: { version: number; status: LeadStatus; lostReason?: LostReason }
): Promise<Lead> {
  return apiFetch<Lead>(`/v1/leads/${id}/status`, {
    method: "PATCH",
    body: { version: input.version, status: input.status, lost_reason: input.lostReason },
  });
}

// assignedToMembershipId: null explicitly unassigns — "melepas" per the
// checklist — undefined would simply omit the field, which the backend
// would treat as "don't touch" instead.
export function updateLeadAssignment(
  id: string,
  input: { version: number; assignedToMembershipId: string | null }
): Promise<Lead> {
  return apiFetch<Lead>(`/v1/leads/${id}/assignment`, {
    method: "PATCH",
    body: { version: input.version, assigned_to_membership_id: input.assignedToMembershipId },
  });
}

export function deleteLead(id: string): Promise<void> {
  return apiFetch<void>(`/v1/leads/${id}`, { method: "DELETE" });
}

export interface Customer {
  id: string;
  name: string;
  email: string | null;
  phone: string | null;
  phone_e164: string | null;
  company: string | null;
  notes: string | null;
  converted_from_lead_id: string;
  converted_by_membership_id: string | null;
  converted_at: string;
  created_at: string;
  updated_at: string;
}

// Only offered when status === "won" (TD §12, checked by the caller via
// canConvertLead in @/lib/lead-status — not duplicated here).
export function convertLead(id: string): Promise<Customer> {
  return apiFetch<Customer>(`/v1/leads/${id}/convert`, { method: "POST" });
}
