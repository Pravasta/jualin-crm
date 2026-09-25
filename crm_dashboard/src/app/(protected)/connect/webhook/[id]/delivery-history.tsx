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
import { STATUS_META } from "@/lib/labels";
import { ListSkeleton } from "../../connect-ui";

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

// Word first, tint second: the status must read without color. Succeeded
// borrows the Menang pair and failed the destructive token (both ≥4.5:1 on
// their tints, #159); in-flight states stay neutral.
const STATUS_CLASSES: Record<DeliveryStatus, string> = {
  pending: "bg-secondary text-secondary-foreground",
  delivering: "bg-secondary text-secondary-foreground",
  succeeded: "",
  failed: "bg-destructive/8 text-destructive",
};

function DeliveryStatusBadge({ status }: { status: DeliveryStatus }) {
  const style = status === "succeeded" ? { background: STATUS_META.won.background, color: STATUS_META.won.color } : undefined;
  return (
    <span
      className={`inline-flex shrink-0 rounded-full px-2.5 py-0.5 text-[12.5px] font-bold ${STATUS_CLASSES[status]}`}
      style={style}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}

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
    return <ListSkeleton label="Memuat riwayat pengiriman" />;
  }

  // One list at every width (#166), not a table: each row carries the
  // server's error text, which is the whole value of this screen (see
  // below), and a table column truncates it or forces sideways scrolling.
  return (
    <div className="flex flex-col gap-3">
      <FormErrorBanner message={error} />

      {deliveries.length === 0 ? (
        <p className="text-[13.5px] text-muted-foreground">
          Belum ada pengiriman. Riwayat akan terisi begitu event pertama terjadi.
        </p>
      ) : (
        <>
          <ul className="overflow-hidden rounded-xl border border-border bg-card">
            {deliveries.map((delivery) => (
              <li
                key={delivery.id}
                className="flex flex-col gap-2 border-b border-border/60 px-4 py-3 last:border-b-0 sm:flex-row sm:items-start sm:justify-between"
              >
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-[14px] font-semibold">
                      {WEBHOOK_EVENT_LABELS[delivery.event_type as WebhookEvent] ?? delivery.event_type}
                    </span>
                    <DeliveryStatusBadge status={delivery.status} />
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-3 text-[13px] text-muted-foreground">
                    <span>{formatDateTimeID(delivery.created_at)}</span>
                    {delivery.attempt > 0 && <span>Percobaan ke-{delivery.attempt}</span>}
                    {delivery.response_status !== null && (
                      <span className="font-mono">HTTP {delivery.response_status}</span>
                    )}
                  </div>
                  {/* The reason is the whole value of this screen: a
                      customer who can see "connection refused" fixes
                      their firewall; one who sees only "Gagal" opens
                      a support ticket. */}
                  {delivery.error && (
                    <div className="mt-1 font-mono text-[12.5px] text-foreground [overflow-wrap:anywhere]">
                      {delivery.error}
                    </div>
                  )}
                  {retryError?.id === delivery.id && (
                    <div className="mt-1 text-[13px] text-destructive">{retryError.message}</div>
                  )}
                </div>
                {/* Offered only where it is valid. The backend rejects the
                    rest with 409, but a button that is always visible and
                    usually fails teaches people to ignore it. */}
                {delivery.status === "failed" && (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={retrying === delivery.id}
                    onClick={() => handleRetry(delivery.id)}
                    className="shrink-0 bg-card sm:h-8"
                  >
                    {retrying === delivery.id ? "Mengirim…" : "Kirim ulang"}
                  </Button>
                )}
              </li>
            ))}
          </ul>

          {totalPages > 1 && (
            <div className="flex flex-wrap items-center justify-between gap-2.5">
              <span className="text-[13px] text-muted-foreground">
                Halaman {page} dari {totalPages} · {total} pengiriman
              </span>
              <div className="flex gap-2">
                <Button type="button" variant="outline" disabled={page <= 1} onClick={() => setPage((p) => p - 1)} className="bg-card md:h-8">
                  Sebelumnya
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                  className="bg-card md:h-8"
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
