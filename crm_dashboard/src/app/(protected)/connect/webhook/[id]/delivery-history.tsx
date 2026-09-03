"use client";

// The delivery history for one endpoint (#103). This is the only place in
// the product where a customer can see what actually happened to their
// data after it left us — so it shows the failure detail, not just a
// status word.
import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import {
  listWebhookDeliveries,
  retryWebhookDelivery,
  WEBHOOK_EVENT_LABELS,
  type DeliveryStatus,
  type WebhookDelivery,
  type WebhookEvent,
} from "@/lib/webhooks";
import { globalMessage } from "@/lib/auth-errors";
import { formatDateTimeID } from "@/lib/date";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

// Four states, four meanings a customer can act on. "delivering" is
// deliberately not called "sedang dikirim… " with an ellipsis of hope —
// it is a claim held by a worker, and it resolves within seconds.
const STATUS_LABELS: Record<DeliveryStatus, string> = {
  pending: "Menunggu",
  delivering: "Sedang dikirim",
  succeeded: "Berhasil",
  failed: "Gagal",
};

const STATUS_CLASSES: Record<DeliveryStatus, string> = {
  pending: "text-muted-foreground",
  delivering: "text-muted-foreground",
  succeeded: "text-foreground/70",
  failed: "text-destructive",
};

export function DeliveryHistory({ endpointId }: { endpointId: string }) {
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [total, setTotal] = useState(0);
  const [perPage, setPerPage] = useState(20);
  const [page, setPage] = useState(1);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Keyed by delivery id: a retry failing on ONE row must not blank the
  // whole table, and must say which row it was about.
  const [retryError, setRetryError] = useState<{ id: string; message: string } | null>(null);
  const [retrying, setRetrying] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    listWebhookDeliveries(endpointId, page, controller.signal)
      .then(({ data, meta }) => {
        setDeliveries(data);
        // Total always from meta.total — data.length is only this page.
        setTotal(meta.total);
        setPerPage(meta.per_page || 20);
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
  }, [endpointId, page, refreshKey]);

  const handleRetry = useCallback(async (deliveryId: string) => {
    setRetrying(deliveryId);
    setRetryError(null);
    try {
      await retryWebhookDelivery(deliveryId);
      // Refetch rather than patching the row in place: the retry resets
      // status and attempt, and the worker may already have moved it on
      // again by the time this returns.
      setRefreshKey((k) => k + 1);
    } catch (err) {
      // 409 delivery_not_retryable lands here when the worker changed the
      // row between render and click. Surfacing it is the point — issue
      // #103's AC is explicit that this must not be a button that goes
      // quiet. The backend's message is shown as-is.
      setRetryError({ id: deliveryId, message: globalMessage(err) });
    } finally {
      setRetrying(null);
    }
  }, []);

  const totalPages = Math.max(1, Math.ceil(total / perPage));

  if (!loaded) {
    return <p className="text-[13px] text-muted-foreground">Memuat riwayat…</p>;
  }

  return (
    <div>
      <FormErrorBanner message={error} />

      {deliveries.length === 0 ? (
        <p className="text-[13px] text-muted-foreground">
          Belum ada pengiriman. Riwayat akan terisi begitu event pertama terjadi.
        </p>
      ) : (
        <>
          <div className="overflow-hidden rounded-lg border border-border bg-background">
            <table className="w-full border-collapse">
              <thead>
                <tr className="bg-muted/40">
                  <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Waktu
                  </th>
                  <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Event
                  </th>
                  <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Status
                  </th>
                  <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Percobaan
                  </th>
                  <th className="px-4 py-2.5" />
                </tr>
              </thead>
              <tbody>
                {deliveries.map((delivery) => (
                  <tr key={delivery.id} className="border-t border-border/70">
                    <td className="px-4 py-2.5 text-[13px] text-foreground/70">
                      {formatDateTimeID(delivery.created_at)}
                    </td>
                    <td className="px-4 py-2.5 text-[13px]">
                      {WEBHOOK_EVENT_LABELS[delivery.event_type as WebhookEvent] ??
                        delivery.event_type}
                    </td>
                    <td className="px-4 py-2.5">
                      <div className={`text-[13px] ${STATUS_CLASSES[delivery.status]}`}>
                        {STATUS_LABELS[delivery.status]}
                        {delivery.response_status !== null && (
                          <span className="ml-1 font-mono text-[11.5px]">
                            HTTP {delivery.response_status}
                          </span>
                        )}
                      </div>
                      {/* The reason is the whole value of this screen: a
                          customer who can see "connection refused" fixes
                          their firewall; one who sees only "Gagal" opens
                          a support ticket. */}
                      {delivery.error && (
                        <div className="text-[11.5px] text-muted-foreground">{delivery.error}</div>
                      )}
                      {retryError?.id === delivery.id && (
                        <div className="mt-1 text-[11.5px] text-destructive">
                          {retryError.message}
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-2.5 text-[13px] text-foreground/70">
                      {delivery.attempt === 0 ? "—" : `ke-${delivery.attempt}`}
                    </td>
                    <td className="px-4 py-2.5 text-right">
                      {/* Offered only where it is valid. The backend
                          rejects the rest with 409, but a button that is
                          always visible and usually fails teaches people
                          to ignore it. */}
                      {delivery.status === "failed" && (
                        <Button
                          type="button"
                          variant="outline"
                          disabled={retrying === delivery.id}
                          onClick={() => handleRetry(delivery.id)}
                        >
                          {retrying === delivery.id ? "Mengirim…" : "Kirim ulang"}
                        </Button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div className="mt-3 flex items-center justify-between">
              <span className="text-[12.5px] text-muted-foreground">
                Halaman {page} dari {totalPages} · {total} pengiriman
              </span>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => p - 1)}
                >
                  Sebelumnya
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                >
                  Berikutnya
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
