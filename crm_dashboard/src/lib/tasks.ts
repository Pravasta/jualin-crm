// Typed wrapper for /v1/leads/{id}/tasks and /v1/tasks/{id} — shape
// verified against crm_be/internal/task/handler_http.go's taskJSON.
import { apiFetch } from "./api-client";

export type TaskStatus = "open" | "done";

export interface Task {
  id: string;
  lead_id: string;
  title: string;
  description: string | null;
  due_at: string | null;
  status: TaskStatus;
  assigned_to_membership_id: string | null;
  completed_at: string | null;
  completed_by_membership_id: string | null;
  version: number;
  created_by_membership_id: string | null;
  created_at: string;
  updated_at: string;
}

export function listTasksByLead(leadId: string, signal?: AbortSignal): Promise<Task[]> {
  return apiFetch<Task[]>(`/v1/leads/${leadId}/tasks`, { signal });
}

export interface CreateTaskInput {
  title: string;
  description?: string;
  dueAt?: string;
  assignedToMembershipId?: string;
}

export function createTask(leadId: string, input: CreateTaskInput): Promise<Task> {
  return apiFetch<Task>(`/v1/leads/${leadId}/tasks`, {
    method: "POST",
    body: {
      title: input.title,
      description: input.description || undefined,
      due_at: input.dueAt || undefined,
      assigned_to_membership_id: input.assignedToMembershipId || undefined,
    },
  });
}

export interface UpdateTaskInput {
  version: number;
  title?: string;
  description?: string;
  dueAt?: string;
  assignedToMembershipId?: string | null;
}

export function updateTask(id: string, input: UpdateTaskInput): Promise<Task> {
  return apiFetch<Task>(`/v1/tasks/${id}`, {
    method: "PATCH",
    body: {
      version: input.version,
      title: input.title,
      description: input.description,
      due_at: input.dueAt,
      assigned_to_membership_id: input.assignedToMembershipId,
    },
  });
}

export function completeTask(id: string, version: number): Promise<Task> {
  return apiFetch<Task>(`/v1/tasks/${id}/complete`, {
    method: "POST",
    body: { version },
  });
}

export function deleteTask(id: string): Promise<void> {
  return apiFetch<void>(`/v1/tasks/${id}`, { method: "DELETE" });
}
