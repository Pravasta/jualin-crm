"use client";

// Admin area — opened rarely, but the only screen in Phase 3 with a real
// decision branch (issue #34). Manager has ActionMembershipList only
// (docs/architecture/authorization.md) — no invite/role-change/
// deactivate, and no ActionInvitationList either, so invitations are
// never fetched for that role at all, not just hidden in the UI.
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { deactivateMembership, listMemberships, updateMembershipRole, type Member } from "@/lib/memberships";
import { listInvitations, revokeInvitation, type Invitation } from "@/lib/invitations";
import { ROLE_LABELS, type Role } from "@/lib/labels";
import { formatDateID } from "@/lib/date";
import { globalMessage, openLeadCountFrom } from "@/lib/auth-errors";
import { canChangeRole, canDeactivate, roleOptionsFor, type TeamActor } from "@/lib/team-permissions";
import { useSession } from "@/lib/session-context";
import { DeactivateMemberDialog } from "./deactivate-member-dialog";
import { InviteMemberDialog } from "./invite-member-dialog";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function TeamScreen() {
  const session = useSession();
  const actor: TeamActor = { membershipId: session.membership_id, role: session.role };
  const canManageTeam = session.role === "owner" || session.role === "admin";

  const [members, setMembers] = useState<Member[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const reload = () => setRefreshKey((k) => k + 1);

  const [inviteOpen, setInviteOpen] = useState(false);
  // Set only once the DEFAULT (reject) deactivate attempt has actually
  // come back 409 membership_has_open_leads — this dialog never opens
  // speculatively, matching "anggota tanpa lead terbuka bisa
  // dinonaktifkan tanpa dialog tambahan" (issue #34 acceptance criterion).
  const [deactivateTarget, setDeactivateTarget] = useState<Member | null>(null);
  const [deactivateOpenLeadCount, setDeactivateOpenLeadCount] = useState(0);
  const [deactivatingId, setDeactivatingId] = useState<string | null>(null);
  const [roleSaving, setRoleSaving] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      listMemberships(controller.signal),
      canManageTeam ? listInvitations(controller.signal) : Promise.resolve<Invitation[]>([]),
    ])
      .then(([memberData, invitationData]) => {
        setMembers(memberData);
        setInvitations(invitationData);
        setError(null);
      })
      .catch((err) => {
        if (!isAbortError(err)) setError(globalMessage(err));
      });
    return () => controller.abort();
  }, [refreshKey, canManageTeam]);

  async function handleRoleChange(member: Member, role: Role) {
    setRoleSaving(member.id);
    setError(null);
    try {
      await updateMembershipRole(member.id, role);
      reload();
    } catch (err) {
      setError(globalMessage(err));
    } finally {
      setRoleSaving(null);
    }
  }

  async function handleRevokeInvitation(invitation: Invitation) {
    setError(null);
    try {
      await revokeInvitation(invitation.id);
      reload();
    } catch (err) {
      setError(globalMessage(err));
    }
  }

  // Always tries the plain (reject) deactivate FIRST. The three-way
  // dialog is not a confirmation step shown up front — it only exists
  // because the backend just said no with a specific reason (issue #34's
  // whole point: this can't be a "Yakin? [Ya]/[Batal]" prompt).
  async function handleDeactivateClick(member: Member) {
    setError(null);
    setDeactivatingId(member.id);
    try {
      await deactivateMembership(member.id);
      reload();
    } catch (err) {
      const count = openLeadCountFrom(err);
      if (count !== null) {
        setDeactivateTarget(member);
        setDeactivateOpenLeadCount(count);
      } else {
        // 403 forbidden (relationship rules), 409 last_owner_cannot_be_removed,
        // etc. — shown apa adanya, no special dialog for these.
        setError(globalMessage(err));
      }
    } finally {
      setDeactivatingId(null);
    }
  }

  return (
    <div>
      <div className="mb-3.5 flex items-center justify-between">
        <h2 className="text-[13.5px] font-semibold">Anggota</h2>
        {canManageTeam && <Button onClick={() => setInviteOpen(true)}>+ Undang anggota</Button>}
      </div>

      <FormErrorBanner message={error} />

      <div className="mb-5.5 overflow-hidden rounded-lg border border-border bg-background">
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-muted/40">
              <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                Nama
              </th>
              <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                Role
              </th>
              <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                Bergabung
              </th>
              <th className="px-4 py-2.5" />
            </tr>
          </thead>
          <tbody>
            {members.map((member) => {
              const row = { membershipId: member.id, role: member.role };
              const editable = canManageTeam && canChangeRole(actor, row);
              const removable = canManageTeam && canDeactivate(actor, row);
              return (
                <tr key={member.id} className="border-t border-border/70">
                  <td className="px-4 py-2.5">
                    <div className="text-[13.5px] font-medium">{member.full_name}</div>
                    <div className="text-[11.5px] text-muted-foreground">{member.email}</div>
                  </td>
                  <td className="px-4 py-2.5 text-[13px]">
                    {editable ? (
                      <select
                        value={member.role}
                        disabled={roleSaving === member.id}
                        onChange={(e) => handleRoleChange(member, e.target.value as Role)}
                        className="h-8 rounded-md border border-input bg-background px-2 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                      >
                        {roleOptionsFor(actor).map((role) => (
                          <option key={role} value={role}>
                            {ROLE_LABELS[role]}
                          </option>
                        ))}
                      </select>
                    ) : (
                      ROLE_LABELS[member.role]
                    )}
                  </td>
                  <td className="px-4 py-2.5 text-[13px] text-foreground/70">
                    {formatDateID(member.created_at)}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    {removable && (
                      <button
                        type="button"
                        disabled={deactivatingId === member.id}
                        onClick={() => handleDeactivateClick(member)}
                        className="rounded-md border border-border px-2.5 py-1 text-xs text-foreground/70 hover:bg-muted disabled:opacity-50"
                      >
                        {deactivatingId === member.id ? "…" : "Nonaktifkan"}
                      </button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {canManageTeam && (
        <>
          <h2 className="mb-2.5 text-[13.5px] font-semibold">Undangan tertunda</h2>
          {invitations.length === 0 ? (
            <p className="text-[13px] text-muted-foreground">Tidak ada undangan tertunda.</p>
          ) : (
            <div className="overflow-hidden rounded-lg border border-border bg-background">
              {invitations.map((invitation) => (
                <div
                  key={invitation.id}
                  className="flex items-center justify-between border-b border-border/60 px-4 py-2.5 last:border-b-0"
                >
                  <div>
                    <div className="text-[13px] font-medium">{invitation.email}</div>
                    <div className="text-[11.5px] text-muted-foreground">
                      {ROLE_LABELS[invitation.role]} · dikirim {formatDateID(invitation.created_at)}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => handleRevokeInvitation(invitation)}
                    className="rounded-md border border-border px-2.5 py-1 text-xs text-foreground/70 hover:bg-muted"
                  >
                    Cabut
                  </button>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      <InviteMemberDialog open={inviteOpen} onOpenChange={setInviteOpen} onCreated={reload} />
      <DeactivateMemberDialog
        member={deactivateTarget}
        openLeadCount={deactivateOpenLeadCount}
        allMembers={members}
        onClose={() => setDeactivateTarget(null)}
        onDeactivated={() => {
          setDeactivateTarget(null);
          reload();
        }}
      />
    </div>
  );
}
