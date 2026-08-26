// Generic envelope types matching docs/architecture/api.md — typed once,
// used everywhere (Aturan #33: {data, meta} success, {error} failure).

export interface Meta {
  page: number;
  per_page: number;
  total: number;
}

export interface Envelope<T> {
  data: T;
  meta?: Meta;
}

export interface ErrorDetail {
  field: string;
  code: string;
}

// ApiErrorBody covers the base error shape plus the optional fields the
// three special-cased codes in TD phase 3 §5 add. Callers narrow via the
// helpers in auth-errors.ts rather than checking these fields directly.
export interface ApiErrorBody {
  code: string;
  message: string;
  details?: ErrorDetail[];
  organizations?: { id: string; name: string }[];
  current?: unknown;
  open_lead_count?: number;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details?: ErrorDetail[];
  readonly body: ApiErrorBody;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.name = "ApiError";
    this.status = status;
    this.code = body.code;
    this.details = body.details;
    this.body = body;
  }
}
