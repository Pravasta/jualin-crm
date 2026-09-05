// Typed wrapper for GET /v1/plans (#125) — the plan comparison screen's
// ONLY source of what each plan offers. This dashboard never computes
// or hardcodes a second copy of plan limits/channels/pricing (Phase 8
// kriteria #6, prd 08.5 kriteria #9); it renders exactly what this
// endpoint sends, shape verified against
// internal/subscription/handler_http.go's list handler.
import { apiFetch } from "./api-client";

export interface PlanCatalogEntry {
  code: string;
  name: string;
  price_label: string;
  limits: {
    leads_per_month: number;
    seats: number;
  };
  channels: Record<string, boolean>;
}

// Not paginated — a plain array via httpx.OK, same shape as
// listWebhookEndpoints/listForms/listAPIKeys: the catalog is fixed at
// three plans, nowhere near needing meta.total.
export function listPlans(signal?: AbortSignal): Promise<PlanCatalogEntry[]> {
  return apiFetch<PlanCatalogEntry[]>("/v1/plans", { signal });
}
