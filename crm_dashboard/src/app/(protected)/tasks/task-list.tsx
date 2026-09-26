"use client";

// Built from the design's TASKS section, but its filtering is entirely
// client-side over a full local array — the real backend paginates
// GET /v1/tasks server-side with status/assigned_to/due_before query
// params (internal/task/handler_http.go's listByOrg), so this is
// server-side filtering from the start, not a port of that logic.
//
// Two things the mockup got that don't carry over as-is:
// - `taskFilterAssignee` matches on the assignee's NAME string; the
//   real `assigned_to` query param is a membership UUID, so the select
//   is built from listMemberships() (id+name), not names harvested off
//   the task list itself.
// - The checkbox is a two-way toggle in the mockup. There's no "reopen
//   task" endpoint (same finding as #33's lead-detail) — it only ever
//   moves open -> done, backed by completeTask(id, version), and
//   disables once done rather than pretending to un-toggle.
//
// Phase 8.6 (#165): one list at every width (rows already stack, no table
// to squeeze); due dates read as calendar words shared with the mobile app
// (lib/due-label.ts); the empty state tells "no tasks yet" apart from "no
// match". The handoff's customer-linked tasks are dummy data — a task
// always belongs to a lead (tasks.lead_id NOT NULL).
import { useEffect, useState } from "react";
import { useRouter, usePathname, useSearchParams } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { completeTask, listTasksByOrg, type Task, type TaskStatus } from "@/lib/tasks";
import { listMemberships, type Member } from "@/lib/memberships";
import { dueLabel, isOverdue } from "@/lib/due-label";
import { useSession } from "@/lib/session-context";
import { cn } from "@/lib/utils";
import { globalMessage, versionConflictCurrent } from "@/lib/auth-errors";

const PER_PAGE = 25;

const STATUS_OPTIONS: { value: "" | TaskStatus; label: string }[] = [
  { value: "", label: "Semua" },
  { value: "open", label: "Belum selesai" },
  { value: "done", label: "Selesai" },
];

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function TaskList() {
  const router = useRouter();
  const pathname = usePathname();
  const session = useSession();
  const searchParams = useSearchParams();

  const assignedTo = searchParams.get("assigned_to") ?? "";
  const statusFilter = (searchParams.get("status") ?? "") as "" | TaskStatus;
  const dueBeforeInput = searchParams.get("due_before") ?? "";
  const page = Math.max(1, Number(searchParams.get("page")) || 1);

  const [tasks, setTasks] = useState<Task[]>([]);
  const [total, setTotal] = useState(0);
  const [members, setMembers] = useState<Member[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loadedKey, setLoadedKey] = useState<string | null>(null);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  function updateParams(patch: Record<string, string | null>) {
    const params = new URLSearchParams(searchParams.toString());
    for (const [key, value] of Object.entries(patch)) {
      if (value === null || value === "") params.delete(key);
      else params.set(key, value);
    }
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  }

  function updateFilterParams(patch: Record<string, string | null>) {
    updateParams({ ...patch, page: null });
  }

  useEffect(() => {
    const controller = new AbortController();
    listMemberships(controller.signal)
      .then(setMembers)
      .catch(() => {
        // Non-fatal: the assignee filter select just won't have options.
      });
    return () => controller.abort();
  }, []);

  const requestKey = JSON.stringify([assignedTo, statusFilter, dueBeforeInput, page, refreshKey]);
  const loading = loadedKey !== requestKey;

  useEffect(() => {
    const controller = new AbortController();
    listTasksByOrg(
      {
        assignedTo: assignedTo || undefined,
        status: statusFilter ? [statusFilter] : undefined,
        dueBefore: dueBeforeInput ? `${dueBeforeInput}T23:59:59.999Z` : undefined,
        page,
        perPage: PER_PAGE,
      },
      controller.signal
    )
      .then(({ data, meta }) => {
        setTasks(data);
        setTotal(meta.total);
        setError(null);
        setLoadedKey(requestKey);
      })
      .catch((err) => {
        if (isAbortError(err)) return;
        setError(globalMessage(err));
        setLoadedKey(requestKey);
      });
    return () => controller.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [assignedTo, statusFilter, dueBeforeInput, page, refreshKey]);

  const membersById = new Map(members.map((m) => [m.id, m]));

  async function handleToggle(task: Task) {
    if (task.status !== "open") return;
    setError(null);
    setTogglingId(task.id);
    try {
      await completeTask(task.id, task.version);
      setRefreshKey((k) => k + 1);
    } catch (err) {
      if (versionConflictCurrent<Task>(err)) {
        // Lower-stakes than a lead conflict (same call as #33's
        // lead-detail) — inline message + refetch, not a second modal.
        setError("Tugas ini sudah diubah di tempat lain. Daftar dimuat ulang.");
        setRefreshKey((k) => k + 1);
      } else {
        setError(globalMessage(err));
      }
    } finally {
      setTogglingId(null);
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const rangeStart = total === 0 ? 0 : (page - 1) * PER_PAGE + 1;
  const rangeEnd = Math.min(page * PER_PAGE, total);
  const hasAnyFilter = assignedTo !== "" || statusFilter !== "" || dueBeforeInput !== "";
  const now = new Date();

  return (
    <div className="flex w-full flex-col gap-3.5 md:gap-4">
      <div className="flex flex-col gap-2.5 md:flex-row md:flex-wrap md:items-center">
        <div role="group" aria-label="Status tugas" className="flex gap-1.5">
          {STATUS_OPTIONS.map((opt) => {
            const active = statusFilter === opt.value;
            return (
              <button
                key={opt.label}
                type="button"
                aria-pressed={active}
                onClick={() => updateFilterParams({ status: opt.value || null })}
                className={cn(
                  "min-h-10 flex-1 rounded-lg border-[1.5px] px-2 text-[13.5px] font-semibold whitespace-nowrap transition-colors md:min-h-9 md:flex-none md:px-3",
                  active ? "border-primary bg-primary text-primary-foreground" : "border-input bg-card text-foreground hover:bg-muted"
                )}
              >
                {opt.label}
              </button>
            );
          })}
        </div>
        <div className="flex flex-col gap-2 sm:flex-row">
          <select
            value={assignedTo}
            onChange={(e) => updateFilterParams({ assigned_to: e.target.value || null })}
            aria-label="Penanggung jawab"
            className="h-11 w-full min-w-0 rounded-lg border border-input bg-card px-3 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 sm:w-auto md:h-9 md:text-[13.5px]"
          >
            <option value="">Semua penanggung jawab</option>
            <option value={session.membership_id}>Tugas saya</option>
            {members
              .filter((m) => m.id !== session.membership_id)
              .map((m) => (
                <option key={m.id} value={m.id}>
                  {m.full_name}
                </option>
              ))}
          </select>
          <label className="flex min-w-0 flex-1 items-center gap-2 md:flex-none">
            <span className="shrink-0 text-[13px] text-muted-foreground">Jatuh tempo s/d</span>
            <input
              type="date"
              value={dueBeforeInput}
              onChange={(e) => updateFilterParams({ due_before: e.target.value || null })}
              className="h-11 min-w-0 flex-1 rounded-lg border border-input bg-card px-2.5 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-9 md:text-[13.5px]"
            />
          </label>
        </div>
        {hasAnyFilter && (
          <button
            type="button"
            onClick={() => router.replace(pathname, { scroll: false })}
            className="self-start text-[13px] font-semibold text-accent-strong underline underline-offset-2 md:self-center"
          >
            Hapus semua filter
          </button>
        )}
      </div>

      {error && <FormErrorBanner message={error} />}

      {loading && tasks.length === 0 && !error && (
        <div aria-busy="true" aria-label="Memuat tugas" className="flex flex-col gap-2">
          {Array.from({ length: 5 }, (_, i) => (
            <div key={i} className="h-16 animate-pulse rounded-[10px] bg-muted" />
          ))}
        </div>
      )}

      {!loading && total === 0 && (
        <div className="rounded-xl border border-dashed border-border bg-card px-5 py-14 text-center">
          {hasAnyFilter ? (
            <>
              <div className="mb-1.5 text-[16px] font-bold">Tidak ada tugas yang cocok</div>
              <p className="mx-auto max-w-[36ch] text-[14px] text-muted-foreground">
                Coba ganti status, penanggung jawab, atau tanggal jatuh tempo.
              </p>
            </>
          ) : (
            <>
              <div className="mb-1.5 text-[16px] font-bold">Belum ada tugas</div>
              <p className="mx-auto max-w-[38ch] text-[14px] text-muted-foreground">
                Tugas dibuat dari halaman detail sebuah lead, dan muncul di sini untuk seluruh organization.
              </p>
            </>
          )}
        </div>
      )}

      {!loading && total > 0 && (
        <>
          <ul className="overflow-hidden rounded-xl border border-border bg-card">
            {tasks.map((task) => {
              const done = task.status === "done";
              const assignee = task.assigned_to_membership_id
                ? membersById.get(task.assigned_to_membership_id)?.full_name
                : null;
              const overdue = !done && !!task.due_at && isOverdue(task.due_at, now);
              return (
                <li
                  key={task.id}
                  className={cn(
                    "flex items-start gap-3 border-b border-border/60 px-4 py-3 last:border-b-0",
                    // Overdue stands out (brief §8.7): a destructive edge as well
                    // as the words, so it survives a quick scan and color loss.
                    overdue && "border-l-[3px] border-l-destructive pl-[13px]"
                  )}
                >
                  <input
                    type="checkbox"
                    checked={done}
                    disabled={done || togglingId === task.id}
                    onChange={() => handleToggle(task)}
                    className="mt-0.5 size-5 shrink-0 accent-[var(--primary)] md:size-4"
                    aria-label={done ? `${task.title} sudah selesai` : `Selesaikan ${task.title}`}
                  />
                  <div className="min-w-0 flex-1">
                    <div
                      className={cn(
                        "text-[14.5px] break-words md:text-[14px]",
                        done ? "text-muted-foreground line-through" : "font-medium text-foreground"
                      )}
                    >
                      {task.title}
                    </div>
                    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[13px] text-muted-foreground">
                      {task.due_at && (
                        <span className={cn(overdue && "font-bold text-destructive")}>
                          {done ? "Selesai" : dueLabel(task.due_at, now)}
                        </span>
                      )}
                      <span>{assignee ?? "Tanpa penanggung jawab"}</span>
                      <Link href={`/leads/${task.lead_id}`} className="font-semibold text-accent-strong hover:underline">
                        Buka lead
                      </Link>
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
          <div className="flex flex-wrap items-center justify-between gap-2.5 text-[13px] text-muted-foreground">
            <span>
              Menampilkan {rangeStart}–{rangeEnd} dari {total} tugas
            </span>
            {totalPages > 1 && (
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  disabled={page <= 1}
                  onClick={() => updateParams({ page: String(page - 1) })}
                  className="h-10 bg-card md:h-8"
                >
                  Sebelumnya
                </Button>
                <span className="hidden sm:inline">
                  Halaman {page} dari {totalPages}
                </span>
                <Button
                  variant="outline"
                  disabled={page >= totalPages}
                  onClick={() => updateParams({ page: String(page + 1) })}
                  className="h-10 bg-card md:h-8"
                >
                  Berikutnya
                </Button>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
