"use client";

// One endpoint: its settings, and what actually happened to the events
// sent to it (#103). The role gate sits above the fetch, same as the list
// screen — typing this URL directly as a Manager makes zero API calls.
//
// No `version` round-trip here: endpointJSON carries none and the
// backend's PATCH is last-write-wins. Aturan #35's optimistic locking
// binds leads and tasks, not this — see lib/webhooks.ts.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FieldError } from "@/components/field-error";
import { FormErrorBanner } from "@/components/form-error-banner";
import {
  getWebhookEndpoint,
  updateWebhookEndpoint,
  WEBHOOK_EVENTS,
  WEBHOOK_EVENT_DESCRIPTIONS,
  WEBHOOK_EVENT_LABELS,
  isWebhookUrlNotAllowed,
  type WebhookEndpoint,
  type WebhookEvent,
} from "@/lib/webhooks";
import { canManageWebhooks } from "@/lib/webhook-permissions";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";
import { formatDateID } from "@/lib/date";
import { useSession } from "@/lib/session-context";
import { DeleteWebhookDialog } from "./delete-webhook-dialog";
import { DeliveryHistory } from "./delivery-history";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}


export function WebhookEditor({ endpointId }: { endpointId: string }) {
  const session = useSession();
  const router = useRouter();
  const canManage = canManageWebhooks(session.role);

  const [endpoint, setEndpoint] = useState<WebhookEndpoint | null>(null);
  const [url, setUrl] = useState("");
  const [description, setDescription] = useState("");
  const [events, setEvents] = useState<WebhookEvent[]>([]);
  const [isActive, setIsActive] = useState(true);

  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  useEffect(() => {
    if (!canManage) return;
    const controller = new AbortController();
    getWebhookEndpoint(endpointId, controller.signal)
      .then((data) => {
        setEndpoint(data);
        setUrl(data.url);
        setDescription(data.description);
        setEvents(data.events);
        setIsActive(data.is_active);
        setLoadError(null);
        setLoaded(true);
      })
      .catch((err) => {
        if (!isAbortError(err)) {
          setLoadError(globalMessage(err));
          setLoaded(true);
        }
      });
    return () => controller.abort();
  }, [canManage, endpointId]);

  function toggleEvent(event: WebhookEvent) {
    setEvents((current) =>
      current.includes(event) ? current.filter((e) => e !== event) : [...current, event]
    );
    setSaved(false);
  }

  async function handleSave() {
    setSaving(true);
    setSaveError(null);
    setFieldErrors({});
    setSaved(false);
    try {
      const updated = await updateWebhookEndpoint(endpointId, {
        url: url.trim(),
        events,
        description: description.trim(),
        is_active: isActive,
      });
      setEndpoint(updated);
      setSaved(true);
    } catch (err) {
      const fields = fieldErrorsFrom(err);
      if (Object.keys(fields).length > 0) {
        setFieldErrors(fields);
      } else if (isWebhookUrlNotAllowed(err)) {
        // Backend's message shown as-is: its vagueness is deliberate
        // (td.md §7 — a specific reason would let someone map our
        // internal network through error messages).
        setFieldErrors({ url: globalMessage(err) });
      } else {
        setSaveError(globalMessage(err));
      }
    } finally {
      setSaving(false);
    }
  }

  if (!canManage) {
    return (
      <p className="text-[13px] text-muted-foreground">
        Pengelolaan webhook tidak tersedia untuk role Anda.
      </p>
    );
  }

  if (!loaded) {
    return <p className="text-[13px] text-muted-foreground">Memuat…</p>;
  }

  if (loadError || !endpoint) {
    return (
      <div>
        <button
          type="button"
          onClick={() => router.push("/connect/webhook")}
          className="mb-3.5 flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground"
        >
          ← Kembali ke Webhook
        </button>
        <FormErrorBanner message={loadError ?? "Endpoint tidak ditemukan."} />
      </div>
    );
  }

  const canSave = url.trim() !== "" && events.length > 0 && !saving;

  return (
    <div>
      <button
        type="button"
        onClick={() => router.push("/connect/webhook")}
        className="mb-3.5 flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground"
      >
        ← Kembali ke Webhook
      </button>

      <div className="mb-4 flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h2 className="text-[13.5px] font-semibold break-all">{endpoint.url}</h2>
          <p className="text-[12.5px] text-muted-foreground">
            Dibuat {formatDateID(endpoint.created_at)} · secret{" "}
            <span className="font-mono">{endpoint.secret_prefix}…</span>
          </p>
        </div>
      </div>

      <section className="mb-6 rounded-lg border border-border bg-background p-4">
        <h3 className="mb-3 text-[13px] font-semibold">Pengaturan</h3>

        <FormErrorBanner message={saveError} />

        <div className="flex flex-col gap-3.5">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="edit-url">URL tujuan</Label>
            <Input
              id="edit-url"
              type="url"
              value={url}
              onChange={(e) => {
                setUrl(e.target.value);
                setSaved(false);
              }}
            />
            <FieldError message={fieldErrors.url} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label>Event yang dikirim</Label>
            {WEBHOOK_EVENTS.map((event) => (
              <label key={event} className="flex items-start gap-2 text-[12.5px]">
                <input
                  type="checkbox"
                  className="mt-0.5"
                  checked={events.includes(event)}
                  onChange={() => toggleEvent(event)}
                />
                <span>
                  <span className="font-medium">{WEBHOOK_EVENT_LABELS[event]}</span>
                  <span className="block text-muted-foreground">
                    {WEBHOOK_EVENT_DESCRIPTIONS[event]}
                  </span>
                </span>
              </label>
            ))}
            <FieldError
              message={events.length === 0 ? "Pilih minimal satu event." : fieldErrors.events}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="edit-description">Keterangan</Label>
            <Input
              id="edit-description"
              placeholder="Sinkronisasi ke sistem gudang"
              value={description}
              onChange={(e) => {
                setDescription(e.target.value);
                setSaved(false);
              }}
            />
            <FieldError message={fieldErrors.description} />
          </div>

          {/* Deactivating stops new deliveries without destroying the
              endpoint or its secret — the reversible option, kept next to
              the settings rather than beside the destructive one. */}
          <label className="flex items-start gap-2 text-[12.5px]">
            <input
              type="checkbox"
              className="mt-0.5"
              checked={isActive}
              onChange={(e) => {
                setIsActive(e.target.checked);
                setSaved(false);
              }}
            />
            <span>
              <span className="font-medium">Aktif</span>
              <span className="block text-muted-foreground">
                Saat dinonaktifkan, Jualin berhenti mengirim event baru ke endpoint ini. Endpoint dan
                secret-nya tetap tersimpan.
              </span>
            </span>
          </label>

          <div className="flex items-center gap-3">
            <Button type="button" disabled={!canSave} onClick={handleSave}>
              {saving ? "Menyimpan…" : "Simpan perubahan"}
            </Button>
            {saved && <span className="text-[12.5px] text-muted-foreground">Tersimpan.</span>}
          </div>
        </div>
      </section>

      <section className="mb-6">
        <h3 className="mb-3 text-[13px] font-semibold">Riwayat pengiriman</h3>
        <DeliveryHistory endpointId={endpointId} />
      </section>

      <section className="rounded-lg border border-destructive/30 bg-background p-4">
        <h3 className="mb-1 text-[13px] font-semibold">Hapus endpoint</h3>
        <p className="mb-3 text-[12.5px] text-muted-foreground">
          Menghapus endpoint tidak bisa dibatalkan. Kalau hanya ingin berhenti sementara, nonaktifkan
          saja di atas.
        </p>
        <Button type="button" variant="destructive" onClick={() => setDeleteOpen(true)}>
          Hapus endpoint
        </Button>
      </section>

      <DeleteWebhookDialog
        endpointId={endpointId}
        endpointUrl={endpoint.url}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={() => router.push("/connect/webhook")}
      />
    </div>
  );
}
