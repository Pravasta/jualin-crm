// The client every screen talks to the API through. Two things make
// this file the hard part of issue #31, not the auth screens
// themselves (TD phase 3 §4):
//
// 1. access_token/refresh_token are HttpOnly (Rule #25) — this code
//    never touches them. credentials: 'include' is what carries them.
// 2. Refresh must be single-flight. A screen with N widgets that all
//    get a 401 at once must trigger exactly ONE call to
//    /v1/auth/refresh — otherwise refresh token rotation (crm_be #10)
//    reads the second call's already-rotated token as a reuse attack
//    and revokes the whole family_id, logging the user out from under
//    them.
import { ApiError, type ApiErrorBody } from "./api-types";
import { CSRF_HEADER_NAME, readCsrfToken } from "./csrf";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
const REFRESH_PATH = "/v1/auth/refresh";

export interface ApiFetchOptions extends Omit<RequestInit, "body"> {
  body?: unknown;
}

interface InternalApiFetchOptions extends ApiFetchOptions {
  /**
   * Set only on the single retry apiFetch performs after a successful
   * refresh. Prevents a request that gets 401 TWICE from ever
   * triggering a second refresh — "satu percobaan, lalu menyerah".
   */
  _isRetry?: boolean;
}

// Module-level — shared by every apiFetch call in this browser tab.
// While non-null, every concurrent 401 awaits this SAME promise instead
// of starting its own refresh. Reset to null once the refresh settles,
// so the NEXT 401 (not one of the ones currently waiting) starts a
// fresh cycle rather than reusing a stale result.
let refreshPromise: Promise<boolean> | null = null;

async function rawFetch(path: string, options: InternalApiFetchOptions): Promise<Response> {
  const method = (options.method ?? "GET").toUpperCase();
  const headers = new Headers(options.headers);

  if (options.body !== undefined && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  // Never on GET/HEAD — TD §4.2's acceptance criterion is as much about
  // the header's ABSENCE on safe methods as its presence on unsafe ones.
  if (method !== "GET" && method !== "HEAD") {
    const csrf = readCsrfToken();
    if (csrf) headers.set(CSRF_HEADER_NAME, csrf);
  }

  return fetch(`${API_BASE_URL}${path}`, {
    ...options,
    method,
    headers,
    credentials: "include",
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });
}

// doRefresh is called at most once per refresh cycle regardless of how
// many callers are waiting — see refreshPromise above. It calls
// rawFetch directly (never apiFetch) so a 401 from refresh itself never
// re-enters the retry logic below.
function doRefresh(): Promise<boolean> {
  refreshPromise ??= rawFetch(REFRESH_PATH, { method: "POST" })
    .then((res) => res.ok)
    .catch(() => false)
    .finally(() => {
      refreshPromise = null;
    });
  return refreshPromise;
}

function redirectToLogin(): void {
  if (typeof window !== "undefined") {
    // A full navigation, not router.push — this module lives outside
    // the React tree (no access to useRouter) and needs to reset all
    // client state unconditionally on session expiry.
    // eslint-disable-next-line @next/next/no-location-assign-relative-destination
    window.location.href = "/login";
  }
}

const sessionExpiredError = new ApiError(401, {
  code: "authentication_required",
  message: "Sesi Anda berakhir. Silakan masuk kembali.",
});

async function parseResponse<T>(response: Response): Promise<T> {
  const text = await response.text();
  const json = text ? JSON.parse(text) : undefined;

  if (!response.ok) {
    const body: ApiErrorBody = json?.error ?? {
      code: "internal_error",
      message: "Terjadi kesalahan internal.",
    };
    throw new ApiError(response.status, body);
  }

  // 204 No Content (e.g. logout) has no body — callers that don't need
  // a return value simply get undefined.
  return (json?.data ?? undefined) as T;
}

export async function apiFetch<T>(
  path: string,
  options: InternalApiFetchOptions = {}
): Promise<T> {
  const response = await rawFetch(path, options);

  const isRefreshCall = path === REFRESH_PATH;
  if (response.status === 401 && !isRefreshCall && !options._isRetry) {
    const refreshed = await doRefresh();
    if (refreshed) {
      return apiFetch<T>(path, { ...options, _isRetry: true });
    }
    redirectToLogin();
    throw sessionExpiredError;
  }

  return parseResponse<T>(response);
}
