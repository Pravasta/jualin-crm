"use client";

// Owner/Admin only — Manager and Employee get NO fetch at all, not just
// a hidden button. "Mengetik URL langsung tidak menampilkan daftar"
// (issue #48 acceptance criterion) means the gate has to sit ABOVE the
// useEffect that calls listAPIKeys, the same way team-screen.tsx skips
// listInvitations for a role without ActionInvitationList.
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { listAPIKeys, type APIKey } from "@/lib/api-keys";
import { listMemberships, type Member } from "@/lib/memberships";
import { canManageAPIKeys, toAPIKeyRow } from "@/lib/api-key-rows";
import { globalMessage } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";
import { CreateAPIKeyDialog } from "./create-api-key-dialog";
import { RevokeAPIKeyDialog } from "./revoke-api-key-dialog";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function APIKeysScreen() {
  const session = useSession();
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
    return (
      <p className="text-[13px] text-muted-foreground">
        Manajemen API key tidak tersedia untuk role Anda.
      </p>
    );
  }

  const loading = loadedKey !== refreshKey;
  const memberName = (membershipId: string | null) =>
    members.find((m) => m.id === membershipId)?.full_name ?? "—";
  const now = new Date();

  return (
    <div>
      <div className="mb-3.5 flex items-center justify-between">
        <h2 className="text-[13.5px] font-semibold">API Key</h2>
        <Button onClick={() => setCreateOpen(true)}>+ Buat kunci baru</Button>
      </div>

      <FormErrorBanner message={error} />

      {loading ? (
        <p className="text-[13px] text-muted-foreground">Memuat…</p>
      ) : keys.length === 0 ? (
        <p className="text-[13px] text-muted-foreground">
          Belum ada API key. Buat satu untuk mulai mengirim lead lewat integrasi eksternal.
        </p>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <table className="w-full border-collapse">
            <thead>
              <tr className="bg-muted/40">
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Kunci
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Scope
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Dibuat oleh
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Terakhir dipakai
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Status
                </th>
                <th className="px-4 py-2.5" />
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => {
                const row = toAPIKeyRow(key, now);
                return (
                  <tr
                    key={row.id}
                    className={`border-t border-border/70 ${row.isRevoked ? "opacity-60" : ""}`}
                  >
                    <td className="px-4 py-2.5">
                      <div className="font-mono text-[12.5px]">{row.keyPrefix}…</div>
                      <div className="text-[11.5px] text-muted-foreground">{row.name}</div>
                    </td>
                    <td className="px-4 py-2.5 text-[13px]">{row.scopeLabels}</td>
                    <td className="px-4 py-2.5 text-[13px] text-foreground/70">
                      {memberName(key.created_by_membership_id)}
                    </td>
                    <td className="px-4 py-2.5 text-[13px] text-foreground/70">{row.lastUsedLabel}</td>
                    <td className="px-4 py-2.5 text-[13px]">
                      <span
                        className={
                          row.isRevoked
                            ? "text-muted-foreground"
                            : "font-medium text-accent-strong"
                        }
                      >
                        {row.statusLabel}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-right">
                      {!row.isRevoked && (
                        <button
                          type="button"
                          onClick={() => setRevokeTarget(key)}
                          className="rounded-md border border-border px-2.5 py-1 text-xs text-foreground/70 hover:bg-muted"
                        >
                          Cabut
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
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
