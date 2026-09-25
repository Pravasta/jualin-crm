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
//
// Phase 8.6 (#167): the handoff's Starter/Tim/Bisnis plans, payment
// history, invoice download, storage quota and self-serve downgrade are
// dummy data — none exists (invoices and payments are out of scope, and
// there is no downgrade path, Phase 8 D4). Every number here still comes
// from /v1/me and /v1/plans.
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { globalMessage } from "@/lib/auth-errors";
import { Check, Minus } from "lucide-react";
import { formatLimit, formatUsage, isUnlimitedLimit, planDisplayName, usageLevel, usageRatio } from "@/lib/plan";
import { cn } from "@/lib/utils";
import { listPlans, type PlanCatalogEntry } from "@/lib/plans";
import { startTestCheckout } from "@/lib/subscription";
import { canChangePlan, canViewSubscription } from "@/lib/subscription-permissions";
import { useSession, useSessionRefresh } from "@/lib/session-context";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

const LEVEL_NOTE = { ok: null, near: "Hampir mencapai batas", full: "Batas tercapai" } as const;

function UsageRow({ label, used, limit }: { label: string; used: number; limit: number }) {
  const level = usageLevel(used, limit);
  const note = LEVEL_NOTE[level];
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-[14px] font-medium">{label}</span>
        <span className={cn("text-[14px] font-bold tabular-nums", level !== "ok" && "text-destructive")}>
          {formatUsage(used, limit)}
        </span>
      </div>
      {!isUnlimitedLimit(limit) && (
        <div
          role="progressbar"
          aria-label={label}
          aria-valuemin={0}
          aria-valuemax={limit}
          aria-valuenow={Math.min(used, limit)}
          className="h-2 overflow-hidden rounded-full bg-muted"
        >
          <div
            className={cn("h-full rounded-full", level === "ok" ? "bg-primary" : "bg-destructive")}
            style={{ width: `${usageRatio(used, limit) * 100}%` }}
          />
        </div>
      )}
      {/* In words, not only in the bar's color. */}
      {note && <span className="text-[12.5px] font-semibold text-destructive">{note}</span>}
    </div>
  );
}

// Channel names as the rest of the product says them (Connect, #166).
const CHANNEL_LABELS: Record<string, string> = { api_key: "API", form: "Formulir", webhook: "Webhook" };

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
    return (
      <div className="mx-auto w-full max-w-[1280px] rounded-xl border border-dashed border-border bg-card px-6 py-12 text-center text-[14px] text-muted-foreground">
        Langganan tidak tersedia untuk role Anda.
      </div>
    );
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
    <div className="mx-auto flex w-full max-w-[1280px] flex-col gap-4">
      <section className="rounded-xl border border-border bg-card p-4 md:p-5">
        <div className="text-[12px] font-bold tracking-[0.05em] text-muted-foreground uppercase">Paket Anda</div>
        <div className="mt-1 text-[24px] leading-tight font-extrabold">{planDisplayName(session.plan.code)}</div>
        <div className="mt-4 grid gap-4 md:grid-cols-2 md:gap-6">
          <UsageRow
            label="Lead bulan ini"
            used={session.plan.usage.leads_this_month}
            limit={session.plan.limits.leads_per_month}
          />
          <UsageRow label="Anggota" used={session.plan.usage.seats_used} limit={session.plan.limits.seats} />
        </div>
      </section>

      <FormErrorBanner message={error} />

      <h2 className="text-[16px] font-bold">Perbandingan paket</h2>

      {loading ? (
        <div aria-busy="true" aria-label="Memuat paket" className="grid gap-3 md:grid-cols-3">
          {Array.from({ length: 3 }, (_, i) => (
            <div key={i} className="h-56 animate-pulse rounded-xl bg-muted" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          {plans.map((plan) => {
            const isCurrent = plan.code === session.plan.code;
            // The test-checkout button only ever targets Pro (crm_be's
            // subscription_admin.go hardcodes the same) — Enterprise
            // never gets one, D4/prd (negotiated, not self-serve).
            const showTestCheckout =
              plan.code === "pro" && !isCurrent && canChange && session.plan.test_checkout_available;

            return (
              <section
                key={plan.code}
                aria-current={isCurrent ? "true" : undefined}
                className={cn(
                  "flex flex-col rounded-xl border bg-card p-4 md:p-5",
                  isCurrent ? "border-2 border-primary" : "border-border"
                )}
              >
                <div className="flex items-center justify-between gap-2">
                  <h3 className="text-[16px] font-bold">{plan.name}</h3>
                  {isCurrent && (
                    <span className="rounded-full bg-accent-tint px-2.5 py-0.5 text-[12px] font-bold text-accent-strong">
                      Paket Anda
                    </span>
                  )}
                </div>
                <div className="mt-1 text-[20px] font-extrabold">{plan.price_label}</div>

                <ul className="mt-4 flex flex-col gap-2 text-[14px]">
                  <li className="flex justify-between gap-3">
                    <span className="text-muted-foreground">Lead / bulan</span>
                    <span className="font-semibold tabular-nums">{formatLimit(plan.limits.leads_per_month)}</span>
                  </li>
                  <li className="flex justify-between gap-3">
                    <span className="text-muted-foreground">Anggota</span>
                    <span className="font-semibold tabular-nums">{formatLimit(plan.limits.seats)}</span>
                  </li>
                  {Object.entries(CHANNEL_LABELS).map(([channel, label]) => {
                    const open = plan.channels[channel] === true;
                    return (
                      <li key={channel} className="flex items-center justify-between gap-3">
                        <span className="text-muted-foreground">Kanal {label}</span>
                        {open ? (
                          <Check className="size-4 text-accent-strong" aria-label="Termasuk" />
                        ) : (
                          <Minus className="size-4 text-muted-foreground" aria-label="Tidak termasuk" />
                        )}
                      </li>
                    );
                  })}
                </ul>

                <div className="mt-auto pt-4">
                  {showTestCheckout && (
                    <Button className="w-full md:h-9" disabled={checkoutLoading} onClick={handleTestCheckout}>
                      {checkoutLoading ? "Memproses…" : "Coba Pro (test)"}
                    </Button>
                  )}

                  {/* Enterprise never gets a BUY button (D4): the price is
                      negotiated, not self-serve. It gets a contact link
                      instead — and only when the backend actually sent
                      one, because a button leading nowhere is worse than
                      no button (issue #125's own "yang tidak boleh
                      terjadi"). rel="noopener noreferrer" because this is
                      the one link on the screen pointing off-product. */}
                  {plan.code === "enterprise" &&
                    (plan.contact_url ? (
                      <a
                        href={plan.contact_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex min-h-11 w-full items-center justify-center rounded-lg border border-input text-[14px] font-semibold text-accent-strong hover:bg-muted md:min-h-9"
                      >
                        Hubungi kami untuk diskusi harga
                      </a>
                    ) : (
                      <p className="text-[13.5px] text-muted-foreground">Hubungi kami untuk diskusi harga.</p>
                    ))}
                </div>
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
