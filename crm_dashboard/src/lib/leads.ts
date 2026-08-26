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
