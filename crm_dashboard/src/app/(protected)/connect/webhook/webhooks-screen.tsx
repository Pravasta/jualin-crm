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
import { BackLink, EmptyCard, ListSkeleton, NotForRole, SectionHeader, tableHeadRow } from "../connect-ui";
import Link from "next/link";
import { BookOpen, Plus } from "lucide-react";
import { cn } from "@/lib/utils";

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
    return <NotForRole>Pengelolaan webhook tidak tersedia untuk role Anda.</NotForRole>;
  }

  const loading = !loaded;

  return (
    <div className="flex w-full flex-col gap-3.5 md:gap-4">
      <BackLink href="/connect" label="Connect" />
      <SectionHeader
        title="Webhook"
        description="Kirim event ke sistem Anda sendiri begitu sesuatu terjadi di Jualin — tanpa perlu menanyakannya berulang kali."
        actions={
          <>
            <Button variant="outline" onClick={() => router.push("/connect/webhook/docs")} className="gap-1.5 bg-card md:h-9">
              <BookOpen className="size-4" aria-hidden />
              Dokumentasi verifikasi
            </Button>
            <Button onClick={() => setCreateOpen(true)} className="gap-1.5 md:h-9 md:px-4">
              <Plus className="size-4" aria-hidden />
              Tambah endpoint
            </Button>
          </>
        }
      />

      <FormErrorBanner message={error} />

      {loading ? (
        <ListSkeleton label="Memuat endpoint" />
      ) : endpoints.length === 0 ? (
        <EmptyCard title="Belum ada endpoint">
          Tambahkan satu untuk mulai mengirim event ke sistem Anda.
        </EmptyCard>
      ) : (
        <>
          <div className="hidden overflow-hidden rounded-xl border border-border bg-card md:block">
            <table className="w-full table-fixed border-collapse text-[14px]">
              <thead>
                <tr className={tableHeadRow}>
                  <th className="px-4 py-2.5">URL</th>
                  <th className="hidden w-56 px-3 py-2.5 lg:table-cell">Event</th>
                  <th className="w-24 px-3 py-2.5">Status</th>
                  <th className="hidden w-32 px-3 py-2.5 lg:table-cell">Dibuat</th>
                  <th className="w-24 px-4 py-2.5" aria-label="Aksi" />
                </tr>
              </thead>
              <tbody>
                {endpoints.map((endpoint) => (
                  <tr
                    key={endpoint.id}
                    className="cursor-pointer border-t border-border/60 hover:bg-muted/50"
                    onClick={() => router.push(`/connect/webhook/${endpoint.id}`)}
                  >
                    <td className="px-4 py-3">
                      <div className="font-semibold [overflow-wrap:anywhere]">{endpoint.url}</div>
                      {/* secret_prefix, never the secret — 8 of 49 characters,
                          enough to tell two endpoints apart and useless alone. */}
                      <div className="font-mono text-[12.5px] text-muted-foreground">{endpoint.secret_prefix}…</div>
                    </td>
                    <td className="hidden px-3 py-3 lg:table-cell">
                      <WebhookEventBadges events={endpoint.events} />
                    </td>
                    <td className="px-3 py-3">
                      <ActiveBadge active={endpoint.is_active} />
                    </td>
                    <td className="hidden px-3 py-3 whitespace-nowrap text-muted-foreground lg:table-cell">
                      {formatDateID(endpoint.created_at)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Link
                        href={`/connect/webhook/${endpoint.id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="font-semibold text-accent-strong hover:underline"
                      >
                        Kelola
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <ul className="flex flex-col gap-2.5 md:hidden">
            {endpoints.map((endpoint) => (
              <li key={endpoint.id}>
                <Link
                  href={`/connect/webhook/${endpoint.id}`}
                  className="block rounded-[10px] border border-border bg-card p-3.5 active:bg-muted/60"
                >
                  <div className="flex items-start justify-between gap-2.5">
                    <div className="min-w-0 text-[14.5px] font-bold [overflow-wrap:anywhere]">{endpoint.url}</div>
                    <ActiveBadge active={endpoint.is_active} />
                  </div>
                  <div className="font-mono text-[12.5px] text-muted-foreground">{endpoint.secret_prefix}…</div>
                  <div className="mt-2">
                    <WebhookEventBadges events={endpoint.events} />
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}

      <CreateWebhookDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => setRefreshKey((k) => k + 1)}
      />
    </div>
  );
}

function ActiveBadge({ active }: { active: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 rounded-full px-2.5 py-0.5 text-[12.5px] font-bold",
        active ? "bg-accent-tint text-accent-strong" : "bg-secondary text-secondary-foreground"
      )}
    >
      {active ? "Aktif" : "Nonaktif"}
    </span>
  );
}
