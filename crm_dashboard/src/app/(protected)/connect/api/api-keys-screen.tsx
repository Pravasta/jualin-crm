"use client";

// Owner/Admin only — Manager and Employee get NO fetch at all, not just
// a hidden button. "Mengetik URL langsung tidak menampilkan daftar"
// (issue #48 acceptance criterion) means the gate has to sit ABOVE the
// useEffect that calls listAPIKeys, the same way team-screen.tsx skips
// listInvitations for a role without ActionInvitationList.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { listAPIKeys, type APIKey } from "@/lib/api-keys";
import { listMemberships, type Member } from "@/lib/memberships";
import { canManageAPIKeys, toAPIKeyRow } from "@/lib/api-key-rows";
import { globalMessage } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";
import { CreateAPIKeyDialog } from "./create-api-key-dialog";
import { RevokeAPIKeyDialog } from "./revoke-api-key-dialog";
import { BackLink, EmptyCard, ListSkeleton, NotForRole, SectionHeader, tableHeadRow } from "../connect-ui";
import { BookOpen, Plus } from "lucide-react";
import { cn } from "@/lib/utils";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function APIKeysScreen() {
  const session = useSession();
  const router = useRouter();
  const canManage = canManageAPIKeys(session.role);

  const [keys, setKeys] = useState<APIKey[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loadedKey, setLoadedKey] = useState(-1);
  const [refreshKey, setRefreshKey] = useState(0);
  const reload = () => setRefreshKey((k) => k + 1);

  const [createOpen, setCreateOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<APIKey | null>(null);

  useEffect(() => {
    if (!canManage) return;
    const controller = new AbortController();
    Promise.all([listAPIKeys(controller.signal), listMemberships(controller.signal)])
      .then(([keyData, memberData]) => {
        setKeys(keyData);
        setMembers(memberData);
        setError(null);
        setLoadedKey(refreshKey);
      })
      .catch((err) => {
        if (!isAbortError(err)) setError(globalMessage(err));
      });
    return () => controller.abort();
  }, [refreshKey, canManage]);

  if (!canManage) {
    return <NotForRole>Manajemen API key tidak tersedia untuk role Anda.</NotForRole>;
  }

  const loading = loadedKey !== refreshKey;
  const memberName = (membershipId: string | null) =>
    members.find((m) => m.id === membershipId)?.full_name ?? "—";
  const now = new Date();

  return (
    <div className="mx-auto flex w-full max-w-[1280px] flex-col gap-3.5 md:gap-4">
      <BackLink href="/connect" label="Connect" />
      <SectionHeader
        title="API key"
        description="Kunci untuk sistem eksternal Anda mengirim lead lewat REST API."
        actions={
          <>
            <Button variant="outline" onClick={() => router.push("/connect/api/docs")} className="gap-1.5 bg-card md:h-9">
              <BookOpen className="size-4" aria-hidden />
              Dokumentasi integrasi
            </Button>
            <Button onClick={() => setCreateOpen(true)} className="gap-1.5 md:h-9 md:px-4">
              <Plus className="size-4" aria-hidden />
              Buat kunci baru
            </Button>
          </>
        }
      />

      <FormErrorBanner message={error} />

      {loading ? (
        <ListSkeleton label="Memuat API key" />
      ) : keys.length === 0 ? (
        <EmptyCard title="Belum ada API key">
          Buat satu untuk mulai mengirim lead lewat integrasi eksternal.
        </EmptyCard>
      ) : (
        <>
          <div className="hidden overflow-hidden rounded-xl border border-border bg-card md:block">
            <table className="w-full table-fixed border-collapse text-[14px]">
              <thead>
                <tr className={tableHeadRow}>
                  <th className="px-4 py-2.5">Kunci</th>
                  <th className="hidden w-32 px-3 py-2.5 lg:table-cell">Scope</th>
                  <th className="hidden w-40 px-3 py-2.5 lg:table-cell">Dibuat oleh</th>
                  <th className="w-40 px-3 py-2.5">Terakhir dipakai</th>
                  <th className="w-24 px-3 py-2.5">Status</th>
                  <th className="w-28 px-4 py-2.5" aria-label="Aksi" />
                </tr>
              </thead>
              <tbody>
                {keys.map((key) => {
                  const row = toAPIKeyRow(key, now);
                  return (
                    <tr key={row.id} className={cn("border-t border-border/60", row.isRevoked && "opacity-60")}>
                      <td className="px-4 py-3">
                        <div className="truncate font-mono text-[13px]">{row.keyPrefix}…</div>
                        <div className="truncate text-[12.5px] text-muted-foreground">{row.name}</div>
                      </td>
                      <td className="hidden px-3 py-3 lg:table-cell">{row.scopeLabels}</td>
                      <td className="hidden truncate px-3 py-3 text-muted-foreground lg:table-cell">
                        {memberName(key.created_by_membership_id)}
                      </td>
                      <td className="px-3 py-3 text-muted-foreground">{row.lastUsedLabel}</td>
                      <td className="px-3 py-3">
                        <KeyStatus revoked={row.isRevoked} label={row.statusLabel} />
                      </td>
                      <td className="px-4 py-3 text-right">
                        {!row.isRevoked && (
                          <Button variant="outline" onClick={() => setRevokeTarget(key)} className="h-8 bg-card">
                            Cabut
                          </Button>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <ul className="flex flex-col gap-2.5 md:hidden">
            {keys.map((key) => {
              const row = toAPIKeyRow(key, now);
              return (
                <li
                  key={row.id}
                  className={cn("rounded-[10px] border border-border bg-card p-3.5", row.isRevoked && "opacity-60")}
                >
                  <div className="flex items-start justify-between gap-2.5">
                    <div className="min-w-0">
                      <div className="truncate text-[15px] font-bold">{row.name}</div>
                      <div className="truncate font-mono text-[13px] text-muted-foreground">{row.keyPrefix}…</div>
                    </div>
                    <KeyStatus revoked={row.isRevoked} label={row.statusLabel} />
                  </div>
                  <div className="mt-2 flex flex-wrap gap-x-3 gap-y-0.5 text-[13px] text-muted-foreground">
                    <span>{row.scopeLabels}</span>
                    <span>Oleh {memberName(key.created_by_membership_id)}</span>
                    <span>{key.last_used_at ? `Dipakai ${row.lastUsedLabel}` : row.lastUsedLabel}</span>
                  </div>
                  {!row.isRevoked && (
                    <Button variant="outline" onClick={() => setRevokeTarget(key)} className="mt-3 w-full bg-card">
                      Cabut
                    </Button>
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}

      <CreateAPIKeyDialog open={createOpen} onOpenChange={setCreateOpen} onCreated={reload} />
      <RevokeAPIKeyDialog
        apiKey={revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onRevoked={() => {
          setRevokeTarget(null);
          reload();
        }}
      />
    </div>
  );
}

// "Aktif" in the accent, "Dicabut" neutral — the word carries it.
function KeyStatus({ revoked, label }: { revoked: boolean; label: string }) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 rounded-full px-2.5 py-0.5 text-[12.5px] font-bold",
        revoked ? "bg-secondary text-secondary-foreground" : "bg-accent-tint text-accent-strong"
      )}
    >
      {label}
    </span>
  );
}
