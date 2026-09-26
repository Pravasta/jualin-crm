"use client";

// Where "hampir seluruh aksi tulis" of the product happens (issue #33).
// Every mutation here follows the same shape: call the API with the
// `version` currently on screen, and on 409 version_conflict, show
// ConflictDialog rather than silently retrying (Aturan #35) — never
// apply the server's `current` payload until the user consciously clicks
// "Muat ulang".
import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  CircleDot,
  MessageCircle,
  Phone,
  Plus,
  StickyNote,
  Trash2,
  X,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { FormErrorBanner } from "@/components/form-error-banner";
import { NoContactBadge } from "@/components/no-contact-badge";
import { StatusBadge } from "@/components/status-badge";
import { ApiError } from "@/lib/api-types";
import {
  convertLead,
  deleteLead,
  getLead,
  updateLeadAssignment,
  updateLeadStatus,
  type Lead,
} from "@/lib/leads";
import { createActivity, listActivities, type Activity, type UserActivityType } from "@/lib/activities";
import { completeTask, deleteTask, listTasksByLead, type Task } from "@/lib/tasks";
import { listMemberships, type Member } from "@/lib/memberships";
import { activityToTimelineEntry, lostReasonDisplayLabel } from "@/lib/activity-text";
import { hasContact } from "@/lib/lead-contact";
import { canConvertLead, hasBeenConverted } from "@/lib/lead-status";
import { SOURCE_LABELS, STATUS_META, type LeadStatus, type LostReason } from "@/lib/labels";
import { cn } from "@/lib/utils";
import { formatDateID } from "@/lib/date";
import { dueLabel, isOverdue } from "@/lib/due-label";
import { globalMessage, isLeadConvertedLocked, versionConflictCurrent } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";
import { ConflictDialog } from "./conflict-dialog";
import { DeleteLeadDialog } from "./delete-lead-dialog";
import { EditLeadDialog } from "./edit-lead-dialog";
import { LeadStatusPanel } from "./lead-status-panel";
import { LostReasonDialog } from "./lost-reason-dialog";
import { NewTaskDialog } from "./new-task-dialog";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

// The three types a person can record by hand — brief §8.5 names them
// Catatan · Telepon · WhatsApp. Icons, not emoji: emoji render differently
// per OS and can't take the token colors.
const NOTE_TYPE_OPTIONS: { type: UserActivityType; label: string; icon: LucideIcon }[] = [
  { type: "note_added", label: "Catatan", icon: StickyNote },
  { type: "call_logged", label: "Telepon", icon: Phone },
  { type: "whatsapp_opened", label: "WhatsApp", icon: MessageCircle },
];

// Timeline marker per activity: the three human types keep their icon on an
// accent tint; everything the system did is a small neutral dot. The eye
// scans the timeline for what PEOPLE did (brief §7.4), so those stand out.
const HUMAN_ICON: Partial<Record<Activity["type"], LucideIcon>> = {
  note_added: StickyNote,
  call_logged: Phone,
  whatsapp_opened: MessageCircle,
};

function formatDateTimeID(iso: string): string {
  return new Date(iso).toLocaleString("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <h2 className="text-[14px] font-bold">{children}</h2>;
}

export function LeadDetail({ leadId }: { leadId: string }) {
  const router = useRouter();
  const session = useSession();

  const [lead, setLead] = useState<Lead | null>(null);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [notFound, setNotFound] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const reload = useCallback(() => setRefreshKey((k) => k + 1), []);

  const [statusError, setStatusError] = useState<string | null>(null);
  const [statusSaving, setStatusSaving] = useState(false);
  const [assignSaving, setAssignSaving] = useState(false);
  const [assignError, setAssignError] = useState<string | null>(null);
  const [convertError, setConvertError] = useState<string | null>(null);
  const [convertSaving, setConvertSaving] = useState(false);

  const [noteType, setNoteType] = useState<UserActivityType>("note_added");
  const [noteDraft, setNoteDraft] = useState("");
  const [noteSaving, setNoteSaving] = useState(false);
  const [noteError, setNoteError] = useState<string | null>(null);

  const [lostDialogOpen, setLostDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [newTaskDialogOpen, setNewTaskDialogOpen] = useState(false);
  const [conflictOpen, setConflictOpen] = useState(false);
  const [taskError, setTaskError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      getLead(leadId, controller.signal),
      listActivities(leadId, controller.signal),
      listTasksByLead(leadId, controller.signal),
      listMemberships(controller.signal),
    ])
      .then(([leadData, activityData, taskData, memberData]) => {
        setLead(leadData);
        setActivities(activityData);
        setTasks(taskData);
        setMembers(memberData);
        setNotFound(false);
        setLoadError(null);
      })
      .catch((err) => {
        if (isAbortError(err)) return;
        if (err instanceof ApiError && err.code === "not_found") setNotFound(true);
        else setLoadError(globalMessage(err));
      });
    return () => controller.abort();
  }, [leadId, refreshKey]);

  const namesById = useMemo(() => new Map(members.map((m) => [m.id, m.full_name])), [members]);
  const timeline = useMemo(
    () =>
      [...activities]
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .map((a) => ({ activity: a, entry: activityToTimelineEntry(a, namesById) })),
    [activities, namesById]
  );

  function handleConflict(err: unknown): boolean {
    if (versionConflictCurrent<Lead>(err)) {
      setConflictOpen(true);
      return true;
    }
    return false;
  }

  function handleReloadConflict() {
    setConflictOpen(false);
    reload();
  }

  async function handleChooseStatus(status: LeadStatus) {
    if (!lead) return;
    if (status === "lost") {
      setLostDialogOpen(true);
      return;
    }
    setStatusError(null);
    setStatusSaving(true);
    try {
      const updated = await updateLeadStatus(lead.id, { version: lead.version, status });
      setLead(updated);
      reload();
    } catch (err) {
      if (!handleConflict(err)) {
        setStatusError(globalMessage(err));
        // A stale screen: the lead was converted since it was loaded. The
        // backend's own sentence is already in the banner; reload so the
        // timeline shows the conversion and the buttons stop being offered.
        if (isLeadConvertedLocked(err)) reload();
      }
    } finally {
      setStatusSaving(false);
    }
  }

  async function handleConfirmLost(reason: LostReason) {
    if (!lead) return;
    try {
      const updated = await updateLeadStatus(lead.id, { version: lead.version, status: "lost", lostReason: reason });
      setLead(updated);
      setLostDialogOpen(false);
      reload();
    } catch (err) {
      if (handleConflict(err)) {
        setLostDialogOpen(false);
        return;
      }
      throw err; // shown inside LostReasonDialog itself
    }
  }

  async function handleAssignChange(membershipId: string) {
    if (!lead) return;
    setAssignError(null);
    setAssignSaving(true);
    try {
      const updated = await updateLeadAssignment(lead.id, {
        version: lead.version,
        assignedToMembershipId: membershipId || null,
      });
      setLead(updated);
      reload();
    } catch (err) {
      if (!handleConflict(err)) setAssignError(globalMessage(err));
    } finally {
      setAssignSaving(false);
    }
  }

  async function handleSubmitNote() {
    if (!lead || !noteDraft.trim()) return;
    setNoteError(null);
    setNoteSaving(true);
    try {
      await createActivity(lead.id, noteType, noteDraft.trim());
      setNoteDraft("");
      reload();
    } catch (err) {
      setNoteError(globalMessage(err));
    } finally {
      setNoteSaving(false);
    }
  }

  async function handleConvert() {
    if (!lead) return;
    setConvertError(null);
    setConvertSaving(true);
    try {
      // No customer detail screen exists yet (#35) — nothing to link to
      // on success, so the return value is intentionally unused.
      await convertLead(lead.id);
      reload();
    } catch (err) {
      // lead_already_converted's backend message ("Lead ini sudah
      // pernah dikonversi...") is already the clear message the
      // acceptance criterion asks for — shown as-is, no special case.
      setConvertError(globalMessage(err));
    } finally {
      setConvertSaving(false);
    }
  }

  async function handleDeleteLead() {
    if (!lead) return;
    await deleteLead(lead.id);
    router.push("/leads");
  }

  async function handleToggleTask(task: Task) {
    setTaskError(null);
    try {
      if (task.status === "open") {
        await completeTask(task.id, task.version);
        reload();
      }
      // Un-completing a task isn't a supported action (no endpoint) —
      // the checkbox only ever moves open -> done.
    } catch (err) {
      if (versionConflictCurrent<Task>(err)) {
        // A task conflict is lower-stakes than a lead conflict — surface
        // it inline and refetch, rather than a second modal pattern.
        setTaskError("Tugas ini sudah diubah di tempat lain. Daftar dimuat ulang.");
        reload();
      } else {
        setTaskError(globalMessage(err));
      }
    }
  }

  async function handleDeleteTask(task: Task) {
    setTaskError(null);
    try {
      await deleteTask(task.id);
      reload();
    } catch (err) {
      setTaskError(globalMessage(err));
    }
  }

  if (notFound) {
    return (
      <div className="flex flex-col items-center gap-3 py-16 text-center">
        <p className="text-sm text-muted-foreground">Lead tidak ditemukan.</p>
        <Button variant="outline" onClick={() => router.push("/leads")}>
          Kembali ke daftar lead
        </Button>
      </div>
    );
  }

  if (loadError) {
    return <FormErrorBanner message={loadError} />;
  }

  if (!lead) {
    return <div className="py-16 text-center text-sm text-muted-foreground">Memuat…</div>;
  }

  const lostReasonLabel = lostReasonDisplayLabel(lead.lost_reason);
  const canDelete = session.role === "owner" || session.role === "admin";
  // The Lead entity itself carries no "already converted" flag —
  // converted_from_lead_id lives on Customer, not Lead (TD §12: the lead
  // is never mutated by conversion). The timeline's own
  // "lead_converted" entry is what's actually checked, so the button
  // disappears once a conversion has genuinely happened rather than
  // relying on convertSaving alone (which resets after a FAILED attempt
  // too).
  const alreadyConverted = hasBeenConverted(activities);
  const canConvert =
    (session.role === "owner" || session.role === "admin") &&
    canConvertLead(lead.status) &&
    !alreadyConverted;

  const meta: { label: string; value: React.ReactNode }[] = [
    { label: "Email", value: lead.email?.trim() || "—" },
    { label: "Telepon", value: lead.phone?.trim() || "—" },
    { label: "Perusahaan", value: lead.company?.trim() || "—" },
    { label: "Sumber", value: SOURCE_LABELS[lead.source] },
    { label: "Masuk", value: formatDateID(lead.created_at) },
  ];

  // Phone: one column in the order the brief sets (§9.2) — header + status,
  // note form, timeline — then assignment, tasks and the two Owner/Admin
  // actions. From 1024px the last group becomes a 300px side column.
  return (
    <div className="flex w-full flex-col gap-3.5">
      <button
        type="button"
        onClick={() => router.push("/leads")}
        className="flex min-h-9 items-center gap-1.5 self-start text-[13.5px] font-medium text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Daftar lead
      </button>

      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[minmax(0,1fr)_300px] lg:gap-5">
        <div className="flex min-w-0 flex-col gap-4">
          {/* Header + status area */}
          <Card>
            <CardContent>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="font-mono text-[13px] text-muted-foreground">#{lead.lead_number}</div>
                  <h1 className="mt-0.5 text-[20px] leading-tight font-bold tracking-tight break-words md:text-[24px]">
                    {lead.name}
                  </h1>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-2">
                  <StatusBadge status={lead.status} size="md" />
                  <Button variant="outline" size="sm" onClick={() => setEditDialogOpen(true)} className="h-9 md:h-7">
                    Ubah
                  </Button>
                </div>
              </div>

              {/* Values wrap, never truncate: a phone number cut to "0857-9999-00…"
                  is useless, and this is the screen people call from. Columns
                  follow the main column's width, which narrows at 1024px when
                  the side column appears. */}
              <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3 lg:grid-cols-2 xl:grid-cols-3 min-[1400px]:grid-cols-5">
                {meta.map((m) => (
                  <div key={m.label} className="min-w-0">
                    <dt className="text-[11.5px] font-semibold tracking-[0.04em] text-muted-foreground uppercase">
                      {m.label}
                    </dt>
                    <dd className="mt-0.5 text-[14px] [overflow-wrap:anywhere]">
                      {m.value}
                    </dd>
                  </div>
                ))}
              </dl>

              {!hasContact(lead) && (
                // Says what to DO, not just what is wrong: the fix is the
                // "Ubah" button next to the name (issue #143).
                <div className="mt-3 flex flex-wrap items-center gap-2 text-[13px] text-muted-foreground">
                  <NoContactBadge />
                  Tambahkan email atau telepon lewat Ubah agar bisa ditindaklanjuti.
                </div>
              )}
              {lead.notes && (
                <p className="mt-3 rounded-lg bg-muted px-3 py-2 text-[13.5px] text-foreground">{lead.notes}</p>
              )}
              {lostReasonLabel && (
                <div
                  className="mt-3 inline-block rounded-md px-2.5 py-1 text-[13px] font-semibold"
                  // The Kalah pair verified in #159: 4.86:1 on its tint.
                  style={{ background: STATUS_META.lost.background, color: STATUS_META.lost.color }}
                >
                  Alasan kalah: {lostReasonLabel}
                </div>
              )}

              <FormErrorBanner message={statusError} />
              <LeadStatusPanel
                status={lead.status}
                converted={alreadyConverted}
                saving={statusSaving}
                onChoose={handleChooseStatus}
              />
            </CardContent>
          </Card>

          {/* Add note */}
          <Card>
            <CardContent className="flex flex-col gap-3">
              <SectionTitle>Tambah catatan</SectionTitle>
              <FormErrorBanner message={noteError} />
              <div role="radiogroup" aria-label="Jenis catatan" className="flex flex-wrap gap-2">
                {NOTE_TYPE_OPTIONS.map((nt) => {
                  const active = noteType === nt.type;
                  const Icon = nt.icon;
                  return (
                    <button
                      key={nt.type}
                      type="button"
                      role="radio"
                      aria-checked={active}
                      onClick={() => setNoteType(nt.type)}
                      className={cn(
                        "flex min-h-10 items-center gap-1.5 rounded-lg border-[1.5px] px-3 text-[13.5px] font-medium transition-colors md:min-h-8",
                        active
                          ? "border-primary bg-primary text-primary-foreground"
                          : "border-input bg-card text-foreground hover:bg-muted"
                      )}
                    >
                      <Icon className="size-4" aria-hidden />
                      {nt.label}
                    </button>
                  );
                })}
              </div>
              <textarea
                value={noteDraft}
                onChange={(e) => setNoteDraft(e.target.value)}
                placeholder="Tulis catatan…"
                aria-label="Isi catatan"
                className="min-h-20 w-full rounded-lg border border-input bg-card px-3 py-2 text-base outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-[14px]"
              />
              <Button
                type="button"
                className="self-start md:h-9 md:px-4"
                disabled={noteSaving || !noteDraft.trim()}
                onClick={handleSubmitNote}
              >
                {noteSaving ? "Menyimpan…" : "Simpan ke timeline"}
              </Button>
            </CardContent>
          </Card>

          {/* Timeline */}
          <Card>
            <CardContent className="flex flex-col gap-2">
              <SectionTitle>Timeline</SectionTitle>
              {timeline.length === 0 && (
                <p className="text-[13px] text-muted-foreground">Belum ada riwayat.</p>
              )}
              <ol className="flex flex-col">
                {timeline.map(({ activity, entry }) => {
                  const Icon = entry.isHuman ? (HUMAN_ICON[activity.type] ?? StickyNote) : CircleDot;
                  return (
                    <li key={activity.id} className="flex gap-3 border-b border-border/60 py-3 last:border-b-0">
                      <span
                        aria-hidden
                        className={cn(
                          "mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full",
                          entry.isHuman ? "bg-accent-tint text-accent-strong" : "bg-muted text-muted-foreground"
                        )}
                      >
                        <Icon className={entry.isHuman ? "size-3.5" : "size-3"} />
                      </span>
                      <div className="min-w-0 flex-1">
                        <div
                          className={cn(
                            "text-[14px] break-words",
                            entry.isHuman ? "font-medium text-foreground" : "text-muted-foreground"
                          )}
                        >
                          {entry.text}
                        </div>
                        <div className="mt-0.5 text-[12px] text-muted-foreground">
                          {entry.isHuman && entry.authorName ? `${entry.authorName} · ` : ""}
                          {formatDateTimeID(activity.created_at)}
                        </div>
                      </div>
                    </li>
                  );
                })}
              </ol>
            </CardContent>
          </Card>
        </div>

        {/* Side column (≥1024px) / end of the page (phone, tablet) */}
        <div className="flex min-w-0 flex-col gap-4">
          <Card>
            <CardContent className="flex flex-col gap-2.5">
              <SectionTitle>Penugasan</SectionTitle>
              <FormErrorBanner message={assignError} />
              <select
                value={lead.assigned_to_membership_id ?? ""}
                disabled={assignSaving}
                onChange={(e) => handleAssignChange(e.target.value)}
                aria-label="Pemilik lead"
                className="h-11 w-full rounded-lg border border-input bg-card px-3 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-9 md:text-[14px]"
              >
                <option value="">Tanpa pemilik</option>
                {members.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.full_name}
                  </option>
                ))}
              </select>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="flex flex-col gap-1">
              <div className="mb-1.5 flex items-center justify-between gap-2">
                <SectionTitle>Tugas</SectionTitle>
                <Button variant="ghost" size="sm" onClick={() => setNewTaskDialogOpen(true)} className="h-9 text-accent-strong md:h-7">
                  <Plus className="size-4" aria-hidden />
                  Tambah
                </Button>
              </div>
              <FormErrorBanner message={taskError} />
              {tasks.length === 0 && (
                <p className="py-1 text-[13px] text-muted-foreground">Belum ada tugas.</p>
              )}
              <ul className="flex flex-col">
                {tasks.map((task) => {
                  const done = task.status === "done";
                  const overdue = !done && !!task.due_at && isOverdue(task.due_at);
                  return (
                    <li key={task.id} className="flex items-start gap-2.5 border-t border-border/60 py-2.5 first:border-t-0">
                      {/* One-way: once done, the box is locked (brief §8.5). The
                          label makes the whole title a 44px touch target. */}
                      <label className="flex min-h-11 min-w-0 flex-1 cursor-pointer items-start gap-2.5 md:min-h-0">
                        <input
                          type="checkbox"
                          checked={done}
                          disabled={done}
                          onChange={() => handleToggleTask(task)}
                          className="mt-0.5 size-4.5 shrink-0 accent-[var(--primary)]"
                        />
                        <span className="min-w-0">
                          <span
                            className={cn(
                              "block text-[14px] break-words",
                              done ? "text-muted-foreground line-through" : "text-foreground"
                            )}
                          >
                            {task.title}
                          </span>
                          <span
                            className={cn(
                              "block text-[12.5px]",
                              overdue ? "font-bold text-destructive" : "text-muted-foreground"
                            )}
                          >
                            {/* Same calendar words as the Tugas list and the
                                phone (lib/due-label.ts, #165). */}
                            {done ? "Selesai" : task.due_at ? dueLabel(task.due_at) : "Tanpa jatuh tempo"}
                          </span>
                        </span>
                      </label>
                      <button
                        type="button"
                        onClick={() => handleDeleteTask(task)}
                        className="flex size-9 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-destructive md:size-7"
                        aria-label={`Hapus tugas ${task.title}`}
                      >
                        <X className="size-4" aria-hidden />
                      </button>
                    </li>
                  );
                })}
              </ul>
            </CardContent>
          </Card>

          {convertError && <FormErrorBanner message={convertError} />}
          {canConvert && (
            <Button
              type="button"
              disabled={convertSaving}
              onClick={handleConvert}
              // White on the Menang green: 5.03:1 (#159).
              className="h-11 text-white md:h-10"
              style={{ background: STATUS_META.won.color }}
            >
              {convertSaving ? "Mengonversi…" : "Konversi menjadi Customer"}
            </Button>
          )}

          {canDelete && (
            // Destructive, and looks it — outlined in the destructive token,
            // apart from the everyday buttons above (brief §5.1 principle 3).
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleteDialogOpen(true)}
              className="h-11 border-destructive/40 bg-card text-destructive hover:bg-destructive/6 hover:text-destructive md:h-9"
            >
              <Trash2 className="size-4" aria-hidden />
              Hapus lead
            </Button>
          )}
        </div>
      </div>

      <LostReasonDialog open={lostDialogOpen} onOpenChange={setLostDialogOpen} onConfirm={handleConfirmLost} />
      <DeleteLeadDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        onConfirm={handleDeleteLead}
      />
      <EditLeadDialog
        open={editDialogOpen}
        onOpenChange={setEditDialogOpen}
        lead={lead}
        onSaved={(updated) => {
          setLead(updated);
          setEditDialogOpen(false);
        }}
        onConflict={() => setConflictOpen(true)}
      />
      <NewTaskDialog
        open={newTaskDialogOpen}
        onOpenChange={setNewTaskDialogOpen}
        leadId={lead.id}
        members={members}
        onCreated={reload}
      />
      <ConflictDialog open={conflictOpen} onReload={handleReloadConflict} />
    </div>
  );
}
