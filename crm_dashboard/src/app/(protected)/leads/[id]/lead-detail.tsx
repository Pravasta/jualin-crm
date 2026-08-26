"use client";

// Where "hampir seluruh aksi tulis" of the product happens (issue #33).
// Every mutation here follows the same shape: call the API with the
// `version` currently on screen, and on 409 version_conflict, show
// ConflictDialog rather than silently retrying (Aturan #35) — never
// apply the server's `current` payload until the user consciously clicks
// "Muat ulang".
import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { FormErrorBanner } from "@/components/form-error-banner";
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
import { canConvertLead, statusTransitionOptions } from "@/lib/lead-status";
import { SOURCE_LABELS, STATUS_META, type LeadStatus, type LostReason } from "@/lib/labels";
import { formatDateID } from "@/lib/date";
import { globalMessage, versionConflictCurrent } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";
import { ConflictDialog } from "./conflict-dialog";
import { DeleteLeadDialog } from "./delete-lead-dialog";
import { EditLeadDialog } from "./edit-lead-dialog";
import { LostReasonDialog } from "./lost-reason-dialog";
import { NewTaskDialog } from "./new-task-dialog";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

const NOTE_TYPE_OPTIONS: { type: UserActivityType; label: string }[] = [
  { type: "note_added", label: "📝 Catatan" },
  { type: "call_logged", label: "📞 Log telepon" },
  { type: "whatsapp_opened", label: "💬 WhatsApp dibuka" },
];

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
      if (!handleConflict(err)) setStatusError(globalMessage(err));
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
        setTaskError("Task ini sudah diubah di tempat lain. Daftar dimuat ulang.");
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

  const statusMeta = STATUS_META[lead.status];
  const lostReasonLabel = lostReasonDisplayLabel(lead.lost_reason);
  const canDelete = session.role === "owner" || session.role === "admin";
  // The Lead entity itself carries no "already converted" flag —
  // converted_from_lead_id lives on Customer, not Lead (TD §12: the lead
  // is never mutated by conversion). The timeline's own
  // "lead_converted" entry is what's actually checked, so the button
  // disappears once a conversion has genuinely happened rather than
  // relying on convertSaving alone (which resets after a FAILED attempt
  // too).
  const alreadyConverted = activities.some((a) => a.type === "lead_converted");
  const canConvert =
    (session.role === "owner" || session.role === "admin") &&
    canConvertLead(lead.status) &&
    !alreadyConverted;

  return (
    <div>
      <button
        type="button"
        onClick={() => router.push("/leads")}
        className="mb-3.5 flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground"
      >
        ← Kembali ke daftar lead
      </button>

      <div className="grid grid-cols-1 items-start gap-5 lg:grid-cols-[1fr_320px]">
        <div className="flex flex-col gap-4">
          {/* Header */}
          <Card>
            <CardContent>
              <div className="mb-1 flex items-start justify-between">
                <div>
                  <div className="mb-0.5 text-xs text-muted-foreground">#{lead.lead_number}</div>
                  <div className="flex items-center gap-2 text-[19px] font-semibold">
                    {lead.name}
                    <button
                      type="button"
                      onClick={() => setEditDialogOpen(true)}
                      className="text-xs font-normal text-accent-strong underline"
                    >
                      Ubah
                    </button>
                  </div>
                </div>
                <span
                  className="inline-block rounded-full px-3 py-1 text-[13px] font-semibold"
                  style={{ background: statusMeta.background, color: statusMeta.color }}
                >
                  {statusMeta.label}
                </span>
              </div>
              <div className="mt-2.5 flex flex-wrap gap-4 text-[13px] text-foreground/70">
                {lead.email && <span>{lead.email}</span>}
                {lead.phone && <span>{lead.phone}</span>}
                <span>Sumber: {SOURCE_LABELS[lead.source]}</span>
                <span>Masuk: {formatDateID(lead.created_at)}</span>
                {lead.company && <span>{lead.company}</span>}
              </div>
              {lead.notes && (
                <div className="mt-2.5 text-[13px] text-muted-foreground">{lead.notes}</div>
              )}
              {lostReasonLabel && (
                <div
                  className="mt-2.5 inline-block rounded-md px-2.5 py-1 text-[12.5px]"
                  style={{ background: "oklch(0.55 0.2 25 / 7%)", color: "oklch(0.5 0.2 25)" }}
                >
                  Alasan kalah: {lostReasonLabel}
                </div>
              )}

              <FormErrorBanner message={statusError} />
              <div className="mt-3.5 flex flex-wrap gap-2">
                {statusTransitionOptions(lead.status).map((opt) => (
                  <button
                    key={opt.status}
                    type="button"
                    disabled={statusSaving}
                    onClick={() => handleChooseStatus(opt.status)}
                    className="h-8 rounded-md px-3 text-[13px] font-medium disabled:opacity-50"
                    style={
                      opt.kind === "step"
                        ? {
                            border: "1px solid oklch(0.56 0.19 41 / 40%)",
                            background: "oklch(0.56 0.19 41 / 8%)",
                            color: "oklch(0.48 0.17 41)",
                          }
                        : {
                            border: "1px solid oklch(0.922 0 0)",
                            background: "#fff",
                            color: "oklch(0.4 0 0)",
                          }
                    }
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* Add note */}
          <Card>
            <CardContent>
              <div className="mb-2.5 text-[13.5px] font-semibold">Tambah catatan</div>
              <FormErrorBanner message={noteError} />
              <div className="mb-2.5 flex gap-1.5">
                {NOTE_TYPE_OPTIONS.map((nt) => (
                  <button
                    key={nt.type}
                    type="button"
                    onClick={() => setNoteType(nt.type)}
                    className="h-7 rounded-md px-2.5 text-[12.5px]"
                    style={{
                      border: `1px solid ${noteType === nt.type ? "oklch(0.56 0.19 41)" : "oklch(0.922 0 0)"}`,
                      background: noteType === nt.type ? "oklch(0.56 0.19 41 / 8%)" : "#fff",
                    }}
                  >
                    {nt.label}
                  </button>
                ))}
              </div>
              <textarea
                value={noteDraft}
                onChange={(e) => setNoteDraft(e.target.value)}
                placeholder="Tulis catatan…"
                className="min-h-16 w-full rounded-md border border-input px-2.5 py-2 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              />
              <Button
                type="button"
                size="sm"
                className="mt-2"
                disabled={noteSaving || !noteDraft.trim()}
                onClick={handleSubmitNote}
              >
                {noteSaving ? "Menyimpan…" : "Simpan"}
              </Button>
            </CardContent>
          </Card>

          {/* Timeline */}
          <Card>
            <CardContent>
              <div className="mb-3 text-[13.5px] font-semibold">Riwayat</div>
              {timeline.length === 0 && (
                <p className="text-[12.5px] text-muted-foreground">Belum ada riwayat.</p>
              )}
              <div className="flex flex-col">
                {timeline.map(({ activity, entry }) => (
                  <div
                    key={activity.id}
                    className="flex gap-2.5 border-b border-border/60 py-2.5 last:border-b-0"
                  >
                    <span
                      className="mt-0.5 flex size-5.5 shrink-0 items-center justify-center rounded-full text-[11px]"
                      style={{
                        background: entry.isHuman ? "oklch(0.56 0.19 41 / 10%)" : "oklch(0.96 0 0)",
                        color: entry.isHuman ? "oklch(0.5 0.17 41)" : "oklch(0.6 0 0)",
                      }}
                    >
                      {entry.isHuman
                        ? activity.type === "call_logged"
                          ? "📞"
                          : activity.type === "whatsapp_opened"
                            ? "💬"
                            : "📝"
                        : "•"}
                    </span>
                    <div className="flex-1">
                      {entry.isHuman && entry.authorName && (
                        <div className="text-[12.5px] font-semibold text-foreground/85">
                          {entry.authorName}
                        </div>
                      )}
                      <div
                        className="text-[13px]"
                        style={{ color: entry.isHuman ? "oklch(0.25 0 0)" : "oklch(0.5 0 0)" }}
                      >
                        {entry.text}
                      </div>
                      <div className="mt-0.5 text-[11.5px] text-muted-foreground">
                        {new Date(activity.created_at).toLocaleString("id-ID", {
                          day: "numeric",
                          month: "short",
                          year: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Right column */}
        <div className="flex flex-col gap-4">
          <Card>
            <CardContent>
              <div className="mb-2.5 text-[13px] font-semibold">Penugasan</div>
              <FormErrorBanner message={assignError} />
              <select
                value={lead.assigned_to_membership_id ?? ""}
                disabled={assignSaving}
                onChange={(e) => handleAssignChange(e.target.value)}
                className="h-8 w-full rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
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
            <CardContent>
              <div className="mb-2.5 flex items-center justify-between">
                <span className="text-[13px] font-semibold">Task</span>
                <button
                  type="button"
                  onClick={() => setNewTaskDialogOpen(true)}
                  className="text-xs font-medium text-accent-strong"
                >
                  + Tambah
                </button>
              </div>
              <FormErrorBanner message={taskError} />
              {tasks.length === 0 && (
                <p className="py-1.5 text-[12.5px] text-muted-foreground">Belum ada task.</p>
              )}
              {tasks.map((task) => {
                const overdue =
                  task.status === "open" && task.due_at && new Date(task.due_at) < new Date();
                return (
                  <div key={task.id} className="flex items-start gap-2 border-t border-border/60 py-2 first:border-t-0">
                    <input
                      type="checkbox"
                      checked={task.status === "done"}
                      disabled={task.status === "done"}
                      onChange={() => handleToggleTask(task)}
                      className="mt-0.5"
                    />
                    <div className="flex-1">
                      <div
                        className="text-[12.5px]"
                        style={{
                          textDecoration: task.status === "done" ? "line-through" : "none",
                          color: task.status === "done" ? "oklch(0.65 0 0)" : "inherit",
                        }}
                      >
                        {task.title}
                      </div>
                      <div
                        className="text-[11px]"
                        style={{ color: overdue ? "oklch(0.55 0.2 25)" : "oklch(0.6 0 0)" }}
                      >
                        {task.due_at ? formatDateID(task.due_at) : "Tanpa jatuh tempo"}
                        {overdue ? " · Terlambat" : ""}
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleDeleteTask(task)}
                      className="text-muted-foreground hover:text-destructive"
                      aria-label="Hapus task"
                    >
                      ×
                    </button>
                  </div>
                );
              })}
            </CardContent>
          </Card>

          {convertError && <FormErrorBanner message={convertError} />}
          {canConvert && (
            <Button
              type="button"
              disabled={convertSaving}
              onClick={handleConvert}
              className="h-9 bg-[oklch(0.5_0.15_145)] text-white hover:bg-[oklch(0.45_0.15_145)]"
            >
              {convertSaving ? "Mengonversi…" : "Konversi menjadi customer"}
            </Button>
          )}

          {canDelete && (
            <button
              type="button"
              onClick={() => setDeleteDialogOpen(true)}
              className="h-8.5 rounded-md text-[13px] font-medium"
              style={{
                border: "1px solid oklch(0.577 0.245 27.325 / 35%)",
                background: "oklch(0.577 0.245 27.325 / 6%)",
                color: "oklch(0.55 0.22 27)",
              }}
            >
              Hapus lead
            </button>
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
