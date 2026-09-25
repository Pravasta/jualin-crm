"use client";

// Admin area — opened rarely, but the only screen in Phase 3 with a real
// decision branch (issue #34). Manager has ActionMembershipList only
// (docs/architecture/authorization.md) — no invite/role-change/
// deactivate, and no ActionInvitationList either, so invitations are
// never fetched for that role at all, not just hidden in the UI.
//
// Phase 8.6 (#165): members are a table from 768px and cards below; each
// member links to their leads. Roles are the four the product has — the
// handoff's "Sales" is dummy data (brief §6: Employee is a role).
import { useEffect, useState } from "react";
import Link from "next/link";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { deactivateMembership, listMemberships, updateMembershipRole, type Member } from "@/lib/memberships";
import { listInvitations, revokeInvitation, type Invitation } from "@/lib/invitations";
import { ROLE_LABELS, type Role } from "@/lib/labels";
import { formatDateID } from "@/lib/date";
import { globalMessage, openLeadCountFrom } from "@/lib/auth-errors";
import { canChangeRole, canDeactivate, roleOptionsFor, type TeamActor } from "@/lib/team-permissions";
import { useSession } from "@/lib/session-context";
import { initialsOf } from "@/lib/nav";
import { cn } from "@/lib/utils";
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

  // Every control a row can carry, rendered once and placed in either the
  // table cell or the card — so the two layouts can't offer different
  // actions to the same role.
  function roleControl(member: Member) {
    const row = { membershipId: member.id, role: member.role };
    if (!(canManageTeam && canChangeRole(actor, row))) return <RoleBadge role={member.role} />;
    return (
      <select
        value={member.role}
        disabled={roleSaving === member.id}
        onChange={(e) => handleRoleChange(member, e.target.value as Role)}
        aria-label={`Role ${member.full_name}`}
        className="h-11 rounded-lg border border-input bg-card px-2.5 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-8 md:text-[13.5px]"
      >
        {roleOptionsFor(actor).map((role) => (
          <option key={role} value={role}>
            {ROLE_LABELS[role]}
          </option>
        ))}
      </select>
    );
  }

  function deactivateControl(member: Member) {
    const row = { membershipId: member.id, role: member.role };
    if (!(canManageTeam && canDeactivate(actor, row))) return null;
    return (
      <Button
        type="button"
        variant="outline"
        disabled={deactivatingId === member.id}
        onClick={() => handleDeactivateClick(member)}
        className="h-11 bg-card text-foreground md:h-8"
      >
        {deactivatingId === member.id ? "Memeriksa…" : "Nonaktifkan"}
      </Button>
    );
  }

  const leadsHref = (member: Member) => `/leads?assigned_to=${member.id}`;

  return (
    <div className="mx-auto flex w-full max-w-[1280px] flex-col gap-3.5 md:gap-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-[15px] font-bold">Anggota</h2>
        {canManageTeam && (
          <Button onClick={() => setInviteOpen(true)} className="gap-1.5 md:h-9 md:px-4">
            <Plus className="size-4" aria-hidden />
            Undang anggota
          </Button>
        )}
      </div>

      <FormErrorBanner message={error} />

      {/* 768px and up */}
      <div className="hidden overflow-hidden rounded-xl border border-border bg-card md:block">
        <table className="w-full table-fixed border-collapse text-[14px]">
          <thead>
            <tr className="bg-muted text-left text-[11.5px] font-bold tracking-[0.05em] text-muted-foreground uppercase">
              <th className="px-4 py-2.5">Anggota</th>
              <th className="w-36 px-3 py-2.5">Role</th>
              <th className="hidden w-32 px-3 py-2.5 lg:table-cell">Bergabung</th>
              <th className="w-28 px-3 py-2.5">Lead</th>
              {canManageTeam && <th className="w-36 px-4 py-2.5" aria-label="Aksi" />}
            </tr>
          </thead>
          <tbody>
            {members.map((member) => (
              <tr key={member.id} className="border-t border-border/60">
                <td className="px-4 py-3">
                  <div className="flex min-w-0 items-center gap-3">
                    <Avatar name={member.full_name} />
                    <div className="min-w-0">
                      <div className="truncate font-semibold">{member.full_name}</div>
                      <div className="truncate text-[12.5px] text-muted-foreground">{member.email}</div>
                    </div>
                  </div>
                </td>
                <td className="px-3 py-3">{roleControl(member)}</td>
                <td className="hidden px-3 py-3 whitespace-nowrap text-muted-foreground lg:table-cell">
                  {formatDateID(member.created_at)}
                </td>
                <td className="px-3 py-3">
                  <Link href={leadsHref(member)} className="font-semibold text-accent-strong hover:underline">
                    Lihat lead
                  </Link>
                </td>
                {canManageTeam && <td className="px-4 py-3 text-right">{deactivateControl(member)}</td>}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Below 768px */}
      <ul className="flex flex-col gap-2.5 md:hidden">
        {members.map((member) => (
          <li key={member.id} className="rounded-[10px] border border-border bg-card p-3.5">
            <div className="flex items-center gap-3">
              <Avatar name={member.full_name} />
              <div className="min-w-0 flex-1">
                <div className="truncate text-[15px] font-bold">{member.full_name}</div>
                <div className="truncate text-[13px] text-muted-foreground">{member.email}</div>
              </div>
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-2.5">
              {roleControl(member)}
              <span className="text-[13px] text-muted-foreground">Bergabung {formatDateID(member.created_at)}</span>
            </div>
            <div className="mt-3 flex flex-wrap items-center justify-between gap-2.5 border-t border-border/60 pt-3">
              <Link href={leadsHref(member)} className="text-[14px] font-semibold text-accent-strong">
                Lihat lead
              </Link>
              {deactivateControl(member)}
            </div>
          </li>
        ))}
      </ul>

      {canManageTeam && (
        <section className="flex flex-col gap-2.5">
          <h2 className="text-[15px] font-bold">Undangan tertunda</h2>
          {invitations.length === 0 ? (
            <p className="text-[13.5px] text-muted-foreground">Tidak ada undangan tertunda.</p>
          ) : (
            <ul className="overflow-hidden rounded-xl border border-border bg-card">
              {invitations.map((invitation) => (
                <li
                  key={invitation.id}
                  className="flex flex-wrap items-center justify-between gap-2.5 border-b border-border/60 px-4 py-3 last:border-b-0"
                >
                  <div className="min-w-0">
                    <div className="text-[14px] font-semibold [overflow-wrap:anywhere]">{invitation.email}</div>
                    <div className="text-[12.5px] text-muted-foreground">
                      {ROLE_LABELS[invitation.role]} · dikirim {formatDateID(invitation.created_at)}
                    </div>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => handleRevokeInvitation(invitation)}
                    className="h-11 bg-card md:h-8"
                  >
                    Cabut
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </section>
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

function Avatar({ name }: { name: string }) {
  return (
    <span
      aria-hidden
      className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent-tint text-[12.5px] font-bold text-accent-strong"
    >
      {initialsOf(name)}
    </span>
  );
}

// Role as text in a tinted pill — Owner in the accent, the rest neutral.
// The word carries the meaning; the tint only groups.
function RoleBadge({ role }: { role: Role }) {
  return (
    <span
      className={cn(
        "inline-flex rounded-full px-2.5 py-0.5 text-[12.5px] font-bold",
        role === "owner" ? "bg-accent-tint text-accent-strong" : "bg-secondary text-secondary-foreground"
      )}
    >
      {ROLE_LABELS[role]}
    </span>
  );
}
