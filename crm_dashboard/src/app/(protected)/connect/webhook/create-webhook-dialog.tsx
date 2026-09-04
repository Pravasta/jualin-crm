"use client";

// Two stages in ONE dialog component, not two separate dialogs — the
// signing secret (Aturan #21: shown exactly once) never leaves this
// component's own local state. It is never passed up to
// webhooks-screen.tsx, never touches that screen's list state, never
// sits in context/localStorage/URL. Closing this dialog is the only
// place it is ever discarded.
//
// Deliberately the same structure as CreateAPIKeyDialog (#48), including
// the onOpenChange choke point. A second, subtly different "show a
// credential once" flow would be a second chance to get it wrong.
import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FieldError } from "@/components/field-error";
import { FormErrorBanner } from "@/components/form-error-banner";
import {
  canCloseCreateDialog,
  createWebhookEndpoint,
  WEBHOOK_EVENTS,
  WEBHOOK_EVENT_DESCRIPTIONS,
  WEBHOOK_EVENT_LABELS,
  isWebhookUrlNotAllowed,
  type CreatedWebhookEndpoint,
  type WebhookEvent,
} from "@/lib/webhooks";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";

type Step = "form" | "reveal";

export function CreateWebhookDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called only AFTER the reveal step closes with confirmedSaved true —
   *  the list reloads once the user has acknowledged saving the secret,
   *  not the instant the endpoint is created. */
  onCreated: () => void;
}) {
  const [step, setStep] = useState<Step>("form");
  const [url, setUrl] = useState("");
  const [description, setDescription] = useState("");
  // Both events pre-selected: an endpoint subscribed to nothing is
  // rejected by the backend anyway (ck_webhook_endpoints_events_not_empty),
  // and "receive everything, narrow later" is the shape someone setting
  // up an integration actually wants.
  const [events, setEvents] = useState<WebhookEvent[]>([...WEBHOOK_EVENTS]);
  const [created, setCreated] = useState<CreatedWebhookEndpoint | null>(null);
  const [copied, setCopied] = useState(false);
  const [confirmedSaved, setConfirmedSaved] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  function reset() {
    setStep("form");
    setUrl("");
    setDescription("");
    setEvents([...WEBHOOK_EVENTS]);
    setCreated(null); // the secret is discarded here
    setCopied(false);
    setConfirmedSaved(false);
    setFieldErrors({});
    setError(null);
  }

  function toggleEvent(event: WebhookEvent) {
    setEvents((current) =>
      current.includes(event) ? current.filter((e) => e !== event) : [...current, event]
    );
  }

  async function handleSubmit(submitEvent: FormEvent) {
    submitEvent.preventDefault();
    setError(null);
    setFieldErrors({});
    setLoading(true);
    try {
      const result = await createWebhookEndpoint({
        url: url.trim(),
        events,
        description: description.trim(),
      });
      setCreated(result);
      setStep("reveal");
    } catch (err) {
      // webhook_url_not_allowed arrives as a plain 400 with a message,
      // not as validation_failed details — so it lands in the banner
      // unless it is steered under the URL field, which is the only
      // place it means anything. The backend's message is shown as-is
      // (never re-translated): its deliberate vagueness is a security
      // decision (td.md §7), not a gap to fill in here.
      //
      // plan_upgrade_required (subscription #113) — the RACE where the
      // organization's plan closed this channel between the card
      // rendering and this click — falls into the generic banner branch
      // below on purpose (lib/plan.ts's isPlanUpgradeRequired names this
      // code; no separate branch is needed since globalMessage(err)
      // already renders the backend's message as-is, same handling
      // #103's delivery_not_retryable gets).
      const fields = fieldErrorsFrom(err);
      if (Object.keys(fields).length > 0) {
        setFieldErrors(fields);
      } else if (isWebhookUrlNotAllowed(err)) {
        setFieldErrors({ url: globalMessage(err) });
      } else {
        setError(globalMessage(err));
      }
    } finally {
      setLoading(false);
    }
  }

  async function handleCopy() {
    if (!created) return;
    try {
      await navigator.clipboard.writeText(created.secret);
      setCopied(true);
    } catch {
      // Clipboard can be unavailable (insecure context, older browser).
      // Not fatal: the secret is still visible and selectable below.
    }
  }

  // The single choke point every close attempt passes through — the X
  // button, Escape, and a backdrop click all route here via base-ui's
  // onOpenChange, not just the footer button. Blocking here rather than
  // only disabling one button is what makes "tutup yang mengharuskan
  // konfirmasi" hold however the user tries to dismiss it.
  function handleOpenChange(next: boolean) {
    if (next) {
      onOpenChange(true);
      return;
    }
    if (!canCloseCreateDialog(step, confirmedSaved)) return; // blocks X, Escape, backdrop
    const wasRevealing = step === "reveal";
    reset();
    onOpenChange(false);
    if (wasRevealing) onCreated();
  }

  const canSubmit = url.trim() !== "" && events.length > 0 && !loading;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent showCloseButton={step !== "reveal"}>
        {step === "form" ? (
          <>
            <DialogHeader>
              <DialogTitle>Tambah endpoint</DialogTitle>
              <DialogDescription>
                Jualin akan mengirim data ke alamat ini setiap kali event yang Anda pilih terjadi.
              </DialogDescription>
            </DialogHeader>

            <form onSubmit={handleSubmit} className="flex flex-col gap-3">
              <FormErrorBanner message={error} />

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="webhook-url">URL tujuan</Label>
                <Input
                  id="webhook-url"
                  required
                  autoFocus
                  type="url"
                  placeholder="https://sistem-anda.com/webhook/jualin"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
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
                  message={
                    events.length === 0 ? "Pilih minimal satu event." : fieldErrors.events
                  }
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="webhook-description">Keterangan (opsional)</Label>
                <Input
                  id="webhook-description"
                  placeholder="Sinkronisasi ke sistem gudang"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
                <FieldError message={fieldErrors.description} />
              </div>

              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => handleOpenChange(false)}>
                  Batal
                </Button>
                <Button type="submit" disabled={!canSubmit}>
                  {loading ? "Menyimpan…" : "Tambah endpoint"}
                </Button>
              </DialogFooter>
            </form>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Signing secret Anda</DialogTitle>
              <DialogDescription>
                Simpan sekarang. Setelah dialog ini ditutup, secret tidak akan ditampilkan lagi —
                tidak di layar ini, tidak di mana pun.
              </DialogDescription>
            </DialogHeader>

            <div className="flex flex-col gap-3">
              <div className="rounded-md border border-border bg-muted/40 p-2.5 font-mono text-[12.5px] break-all select-all">
                {created?.secret}
              </div>

              <Button type="button" variant="outline" onClick={handleCopy}>
                {copied ? "Tersalin" : "Salin secret"}
              </Button>

              {/* This is what the secret is FOR — without it, someone
                  receiving our POST has no way to tell it came from us.
                  Saying so here is more useful than a generic "keep it
                  safe", because it explains what breaks if it is lost. */}
              <p className="text-[12.5px] text-muted-foreground">
                Sistem penerima memakai secret ini untuk memastikan sebuah kiriman benar-benar
                berasal dari Jualin, lewat header{" "}
                <span className="font-mono">X-Jualin-Signature</span>. Tanpa secret, penerima tidak
                punya cara membedakan kiriman kami dari kiriman siapa pun.
              </p>

              <label className="flex items-start gap-2 text-[12.5px] text-muted-foreground">
                <input
                  type="checkbox"
                  checked={confirmedSaved}
                  onChange={(e) => setConfirmedSaved(e.target.checked)}
                  className="mt-0.5"
                />
                Saya sudah menyimpan secret ini di tempat yang aman.
              </label>
            </div>

            <DialogFooter>
              <Button type="button" disabled={!confirmedSaved} onClick={() => handleOpenChange(false)}>
                Selesai
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

