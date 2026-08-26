// Typed wrapper for /v1/customers — shapes verified against
// crm_be/internal/customer/handler_http.go's customerJSON. The
// `Customer` type itself lives in @/lib/leads (added there in #33 for
// convertLead's return value) rather than duplicated here.
import { apiFetch, apiFetchList } from "./api-client";
import type { Meta } from "./api-types";
import type { Customer } from "./leads";

export interface ListCustomersFilter {
  q?: string;
  page?: number;
  perPage?: number;
}

// Only q/page/per_page — internal/customer/usecase.go's ListInput has
// no other fields (no status/source/date-range like leads).
export function listCustomers(
  filter: ListCustomersFilter,
  signal?: AbortSignal
): Promise<{ data: Customer[]; meta: Meta }> {
  const params = new URLSearchParams();
  if (filter.q) params.set("q", filter.q);
  if (filter.page) params.set("page", String(filter.page));
  if (filter.perPage) params.set("per_page", String(filter.perPage));
  const qs = params.toString();
  return apiFetchList<Customer>(`/v1/customers${qs ? `?${qs}` : ""}`, { signal });
}

export function getCustomer(id: string, signal?: AbortSignal): Promise<Customer> {
  return apiFetch<Customer>(`/v1/customers/${id}`, { signal });
}

// No `version` field — internal/customer/entity.go's UpdateInput has
// none: "customers aren't edited from offline mobile (TD §8), so
// there's no write conflict to detect." No ConflictDialog needed here,
// unlike every lead/task write.
export interface UpdateCustomerInput {
  name?: string;
  email?: string | null;
  phone?: string | null;
  company?: string | null;
  notes?: string | null;
}

export function updateCustomer(id: string, input: UpdateCustomerInput): Promise<Customer> {
  return apiFetch<Customer>(`/v1/customers/${id}`, { method: "PATCH", body: input });
}

export function deleteCustomer(id: string): Promise<void> {
  return apiFetch<void>(`/v1/customers/${id}`, { method: "DELETE" });
}
