// Typed wrapper for POST /v1/subscription/test-checkout (#124/#125).
// The route only exists at all when the deployment enables it
// (SUBSCRIPTION_TEST_CHECKOUT=true, reported via plan.test_checkout_available
// on GET /v1/me) — calling this when that flag is false gets a 404, the
// same "route not registered" shape as a disabled webhook admin surface.
import { apiFetch } from "./api-client";

export interface TestCheckoutResult {
  plan_code: string;
}

export function startTestCheckout(): Promise<TestCheckoutResult> {
  return apiFetch<TestCheckoutResult>("/v1/subscription/test-checkout", { method: "POST" });
}
