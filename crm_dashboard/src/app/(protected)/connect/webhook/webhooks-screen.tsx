"use client";

// Outbound webhook management — the list screen (#103). Owner/Admin only:
// Manager and Employee get NO fetch at all (the gate sits above the
// useEffect that calls listWebhookEndpoints), the same shape
// forms-screen.tsx and api-keys-screen.tsx use — nol panggilan API, not
// just a hidden button.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { listWebhookEndpoints, type WebhookEndpoint } from "@/lib/webhooks";
import { canManageWebhooks } from "@/lib/webhook-permissions";
import { globalMessage } from "@/lib/auth-errors";
import { formatDateID } from "@/lib/date";
import { useSession } from "@/lib/session-context";
import { CreateWebhookDialog } from "./create-webhook-dialog";
import { WebhookEventBadges } from "./webhook-event-badges";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function WebhooksScreen() {
  const session = useSession();
  const router = useRouter();
  const canManage = canManageWebhooks(session.role);

  const [endpoints, setEndpoints] = useState<WebhookEndpoint[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  // Bumped after a create so the effect refetches — the newly created
  // endpoint is only added to the list once its secret dialog is closed
  // and acknowledged.
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    if (!canManage) return;
    const controller = new AbortController();
    listWebhookEndpoints(controller.signal)
      .then((data) => {
        setEndpoints(data);
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
  }, [canManage, refreshKey]);

  if (!canManage) {
    return (
      <p className="text-[13px] text-muted-foreground">
        Pengelolaan webhook tidak tersedia untuk role Anda.
      </p>
    );
  }

  const loading = !loaded;

  return (
    <div>
      <button
        type="button"
        onClick={() => router.push("/connect")}
        className="mb-3.5 flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground"
      >
        ← Kembali ke Connect
      </button>

      <div className="mb-3.5 flex items-center justify-between">
        <div>
          <h2 className="text-[13.5px] font-semibold">Webhook</h2>
          <p className="text-[12.5px] text-muted-foreground">
            Kirim event ke sistem Anda sendiri begitu sesuatu terjadi di Jualin — tanpa perlu
            menanyakannya berulang kali.{" "}
            <button
              type="button"
              onClick={() => router.push("/connect/webhook/docs")}
              className="text-accent-strong underline"
            >
              Dokumentasi verifikasi
            </button>
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>+ Tambah endpoint</Button>
      </div>

      <FormErrorBanner message={error} />

      {loading ? (
        <p className="text-[13px] text-muted-foreground">Memuat…</p>
      ) : endpoints.length === 0 ? (
        <p className="text-[13px] text-muted-foreground">
          Belum ada endpoint. Tambahkan satu untuk mulai mengirim event ke sistem Anda.
        </p>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <table className="w-full border-collapse">
            <thead>
              <tr className="bg-muted/40">
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  URL
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Event
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Status
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Dibuat
                </th>
                <th className="px-4 py-2.5" />
              </tr>
            </thead>
            <tbody>
              {endpoints.map((endpoint) => (
                <tr
                  key={endpoint.id}
                  className="cursor-pointer border-t border-border/70 hover:bg-muted/30"
                  onClick={() => router.push(`/connect/webhook/${endpoint.id}`)}
                >
                  <td className="px-4 py-2.5">
                    <div className="text-[13px] font-medium break-all">{endpoint.url}</div>
                    {/* secret_prefix, never the secret — 8 of 49 characters,
                        enough to tell two endpoints apart and useless alone. */}
                    <div className="font-mono text-[11.5px] text-muted-foreground">
                      {endpoint.secret_prefix}…
                    </div>
                  </td>
                  <td className="px-4 py-2.5">
                    <WebhookEventBadges events={endpoint.events} />
                  </td>
                  <td className="px-4 py-2.5">
                    <span
                      className={
                        endpoint.is_active
                          ? "text-[12.5px] text-foreground/70"
                          : "text-[12.5px] text-muted-foreground"
                      }
                    >
                      {endpoint.is_active ? "Aktif" : "Nonaktif"}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-[13px] text-foreground/70">
                    {formatDateID(endpoint.created_at)}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <span className="text-[13px] text-accent-strong underline">Kelola</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <CreateWebhookDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => setRefreshKey((k) => k + 1)}
      />
    </div>
  );
}
