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
import { useEffect, useState } from "react";
import { useRouter, usePathname, useSearchParams } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { completeTask, listTasksByOrg, type Task, type TaskStatus } from "@/lib/tasks";
import { listMemberships, type Member } from "@/lib/memberships";
import { formatDateID } from "@/lib/date";
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
        setError("Task ini sudah diubah di tempat lain. Daftar dimuat ulang.");
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

  return (
    <div>
      <div className="mb-3.5 flex flex-wrap items-center gap-2">
        <select
          value={assignedTo}
          onChange={(e) => updateFilterParams({ assigned_to: e.target.value || null })}
          className="h-8.5 rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <option value="">Semua penanggung jawab</option>
          {members.map((m) => (
            <option key={m.id} value={m.id}>
              {m.full_name}
            </option>
          ))}
        </select>
        <input
          type="date"
          value={dueBeforeInput}
          onChange={(e) => updateFilterParams({ due_before: e.target.value || null })}
          aria-label="Jatuh tempo sebelum"
          className="h-8.5 rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        />
        <div className="flex items-center gap-1.5">
          {STATUS_OPTIONS.map((opt) => {
            const active = statusFilter === opt.value;
            return (
              <button
                key={opt.label}
                type="button"
                onClick={() => updateFilterParams({ status: opt.value || null })}
                className="h-8.5 rounded-md border-[1.5px] px-3 text-[13px] font-medium transition-colors"
                style={{
                  borderColor: active ? "oklch(0.56 0.19 41)" : "oklch(0.922 0 0)",
                  background: active ? "oklch(0.56 0.19 41 / 8%)" : "#fff",
                  color: active ? "oklch(0.48 0.17 41)" : "oklch(0.4 0 0)",
                }}
              >
                {opt.label}
              </button>
            );
          })}
        </div>
      </div>

      {error && <FormErrorBanner message={error} />}

      {!loading && total === 0 && (
        <div className="rounded-lg border border-dashed border-muted-foreground/30 bg-background px-6 py-12 text-center text-[13px] text-muted-foreground">
          Tidak ada task yang cocok dengan filter saat ini.
        </div>
      )}

      {loading && tasks.length === 0 && !error && (
        <div className="rounded-lg border border-border bg-background px-6 py-12 text-center text-sm text-muted-foreground">
          Memuat…
        </div>
      )}

      {!loading && total > 0 && (
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          {tasks.map((task) => {
            const assignee = task.assigned_to_membership_id
              ? membersById.get(task.assigned_to_membership_id)?.full_name
              : null;
            const overdue = task.status === "open" && !!task.due_at && new Date(task.due_at) < new Date();
            return (
              <div
                key={task.id}
                className="flex items-center gap-3 border-b border-border/60 px-4 py-2.75 last:border-b-0"
              >
                <input
                  type="checkbox"
                  checked={task.status === "done"}
                  disabled={task.status === "done" || togglingId === task.id}
                  onChange={() => handleToggle(task)}
                  className="size-4"
                  aria-label={`Selesaikan ${task.title}`}
                />
                <div className="min-w-0 flex-1">
                  <div
                    className="text-[13px]"
                    style={{
                      textDecoration: task.status === "done" ? "line-through" : "none",
                      color: task.status === "done" ? "oklch(0.65 0 0)" : "inherit",
                    }}
                  >
                    {task.title}
                  </div>
                  <div className="mt-0.5 text-[12px] text-muted-foreground">
                    <Link href={`/leads/${task.lead_id}`} className="underline">
                      Lead
                    </Link>
                    {assignee ? ` · ${assignee}` : ""}
                  </div>
                </div>
                {task.due_at && (
                  <div
                    className="shrink-0 text-[12.5px]"
                    style={{
                      color: overdue ? "oklch(0.55 0.2 25)" : "oklch(0.5 0 0)",
                      fontWeight: overdue ? 600 : 400,
                    }}
                  >
                    {formatDateID(task.due_at)}
                    {overdue ? " · Terlambat" : ""}
                  </div>
                )}
              </div>
            );
          })}
          <div className="flex items-center justify-between border-t border-border px-3.5 py-2.5 text-[12.5px] text-muted-foreground">
            <span>
              Menampilkan {rangeStart}–{rangeEnd} dari {total} task
            </span>
            {totalPages > 1 && (
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => updateParams({ page: String(page - 1) })}
                >
                  Sebelumnya
                </Button>
                <span>
                  Halaman {page} dari {totalPages}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= totalPages}
                  onClick={() => updateParams({ page: String(page + 1) })}
                >
                  Berikutnya
                </Button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
