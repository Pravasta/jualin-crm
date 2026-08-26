"use client";

import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
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
import { createTask } from "@/lib/tasks";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";
import type { Member } from "@/lib/memberships";

// The design's own "+ Tambah" creates a hardcoded "Task baru" with no
// form at all (visual-only placeholder in the mockup's own code). A real
// task needs at least a title — this is a straightforward addition
// following the same modal pattern as new-lead-dialog.tsx, not a design
// deviation worth belaboring.
export function NewTaskDialog({
  open,
  onOpenChange,
  leadId,
  members,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  leadId: string;
  members: Member[];
  onCreated: () => void;
}) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [dueAt, setDueAt] = useState("");
  const [assignedTo, setAssignedTo] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  function reset() {
    setTitle("");
    setDescription("");
    setDueAt("");
    setAssignedTo("");
    setFieldErrors({});
    setError(null);
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldErrors({});
    setLoading(true);
    try {
      await createTask(leadId, {
        title,
        description: description || undefined,
        dueAt: dueAt ? `${dueAt}T00:00:00.000Z` : undefined,
        assignedToMembershipId: assignedTo || undefined,
      });
      reset();
      onOpenChange(false);
      onCreated();
    } catch (err) {
      const fields = fieldErrorsFrom(err);
      if (Object.keys(fields).length > 0) setFieldErrors(fields);
      else setError(globalMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Task baru</DialogTitle>
          <DialogDescription>Tambahkan follow-up untuk lead ini.</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <FormErrorBanner message={error} />

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="task-title">Judul</Label>
            <Input
              id="task-title"
              required
              autoFocus
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
            <FieldError message={fieldErrors.title} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="task-description">Deskripsi</Label>
            <Textarea
              id="task-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="task-due">Jatuh tempo</Label>
            <input
              id="task-due"
              type="date"
              value={dueAt}
              onChange={(e) => setDueAt(e.target.value)}
              className="h-8 rounded-md border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="task-assignee">Penanggung jawab</Label>
            <select
              id="task-assignee"
              value={assignedTo}
              onChange={(e) => setAssignedTo(e.target.value)}
              className="h-8 rounded-md border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <option value="">Tanpa penanggung jawab</option>
              {members.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.full_name}
                </option>
              ))}
            </select>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Batal
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? "Menyimpan…" : "Buat task"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
