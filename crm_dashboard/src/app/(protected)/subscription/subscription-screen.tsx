"use client";

// Langganan screen (#125) — paket aktif, pemakaian, and the three-plan
// comparison. Visible to every role (same as Connect, lib/nav.ts's own
// comment): Manager/Employee can open /subscription and see "tidak
// tersedia untuk role Anda", the gate sits above the fetch (canManageX
// pattern used by webhooks-screen.tsx/forms-screen.tsx/api-keys-screen.tsx),
// not merely a hidden nav item.
//
// "Paket aktif" and "Pemakaian" read session.plan directly — GET /v1/me
// already carries limits/usage (subscription TD §7), so there is
// nothing to fetch for that part. Only the comparison table needs its
// own call (GET /v1/plans, #125's own endpoint): what OTHER plans offer
// isn't in session.plan at all, and #125/Phase 8 kriteria #6 forbid a
// second, TypeScript-side copy of that.
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { FormErrorBanner } from "@/components/form-error-banner";
import { globalMessage } from "@/lib/auth-errors";
import { formatLimit, formatUsage, isUnlimitedLimit, planDisplayName, usageRatio } from "@/lib/plan";
import { listPlans, type PlanCatalogEntry } from "@/lib/plans";
import { startTestCheckout } from "@/lib/subscription";
import { canChangePlan, canViewSubscription } from "@/lib/subscription-permissions";
import { useSession, useSessionRefresh } from "@/lib/session-context";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

function UsageRow({ label, used, limit }: { label: string; used: number; limit: number }) {
  return (
    <div className="mb-3 last:mb-0">
      <div className="mb-1 flex items-baseline justify-between">
        <span className="text-[12.5px] text-muted-foreground">{label}</span>
        <span className="text-[12.5px] font-medium">{formatUsage(used, limit)}</span>
      </div>
      {!isUnlimitedLimit(limit) && (
        <div className="h-1.5 overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-accent-strong"
            style={{ width: `${usageRatio(used, limit) * 100}%` }}
          />
        </div>
      )}
    </div>
  );
}

export function SubscriptionScreen() {
  const session = useSession();
  const refreshSession = useSessionRefresh();
  const canView = canViewSubscription(session.role);
  const canChange = canChangePlan(session.role);

  const [plans, setPlans] = useState<PlanCatalogEntry[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [checkoutLoading, setCheckoutLoading] = useState(false);

  useEffect(() => {
    if (!canView) return;
    const controller = new AbortController();
    listPlans(controller.signal)
      .then((data) => {
        setPlans(data);
        setError(null);
        setLoaded(true);
      })
      .catch((err) => {
        if (!isAbortError(err)) {
          setError(globalMessage(err));
          setLoaded(true);
        }
      });
    return () => controller.abort();
  }, [canView]);

  if (!canView) {
    return <p className="text-[13px] text-muted-foreground">Langganan tidak tersedia untuk role Anda.</p>;
  }

  async function handleTestCheckout() {
    setError(null);
    setCheckoutLoading(true);
    try {
      await startTestCheckout();
      await refreshSession();
    } catch (err) {
      setError(globalMessage(err));
    } finally {
      setCheckoutLoading(false);
    }
  }

  const loading = !loaded;

  return (
    <div>
      <div className="mb-5.5 rounded-lg border border-border bg-background p-4">
        <div className="mb-3.5 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
          Paket Anda
        </div>
        <div className="mb-3.5 text-[15px] font-semibold">{planDisplayName(session.plan.code)}</div>

        <UsageRow
          label="Lead bulan ini"
          used={session.plan.usage.leads_this_month}
          limit={session.plan.limits.leads_per_month}
        />
        <UsageRow label="Anggota" used={session.plan.usage.seats_used} limit={session.plan.limits.seats} />
      </div>

      <FormErrorBanner message={error} />

      <h2 className="mb-2.5 text-[13.5px] font-semibold">Perbandingan paket</h2>

      {loading ? (
        <p className="text-[13px] text-muted-foreground">Memuat…</p>
      ) : (
        <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-3">
          {plans.map((plan) => {
            const isCurrent = plan.code === session.plan.code;
            // The test-checkout button only ever targets Pro (crm_be's
            // subscription_admin.go hardcodes the same) — Enterprise
            // never gets one, D4/prd (negotiated, not self-serve).
            const showTestCheckout =
              plan.code === "pro" && !isCurrent && canChange && session.plan.test_checkout_available;

            return (
              <Card key={plan.code} className={isCurrent ? "ring-2 ring-accent-strong" : undefined}>
                <CardContent>
                  <div className="mb-1 flex items-center gap-2">
                    <span className="text-[13.5px] font-semibold">{plan.name}</span>
                    {isCurrent && (
                      <span className="rounded-full bg-accent-tint px-2 py-0.5 text-[10.5px] font-medium text-accent-strong">
                        Paket Anda
                      </span>
                    )}
                  </div>
                  <div className="mb-3 text-[13px] text-muted-foreground">{plan.price_label}</div>

                  <dl className="mb-3 space-y-1 text-[12.5px]">
                    <div className="flex justify-between">
                      <dt className="text-muted-foreground">Lead / bulan</dt>
                      <dd className="font-medium">{formatLimit(plan.limits.leads_per_month)}</dd>
                    </div>
                    <div className="flex justify-between">
                      <dt className="text-muted-foreground">Anggota</dt>
                      <dd className="font-medium">{formatLimit(plan.limits.seats)}</dd>
                    </div>
                  </dl>

                  {showTestCheckout && (
                    <Button size="sm" className="w-full" disabled={checkoutLoading} onClick={handleTestCheckout}>
                      {checkoutLoading ? "Memproses…" : "Coba Pro (test)"}
                    </Button>
                  )}

                  {/* Enterprise never gets a buy button (D4): the price is
                      negotiated, not self-serve, and a button leading
                      nowhere is worse than no button (issue #125's own
                      "yang tidak boleh terjadi"). The contact channel
                      itself is still a placeholder pending the product
                      owner (docs/issues/125) — text only until then. */}
                  {plan.code === "enterprise" && (
                    <p className="text-[12.5px] text-muted-foreground">Hubungi kami untuk diskusi harga.</p>
                  )}
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
