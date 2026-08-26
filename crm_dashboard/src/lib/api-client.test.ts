// The concurrency test below is this file's reason to exist — issue
// #31's acceptance criterion is "N request paralel yang menerima 401
// menghasilkan TEPAT SATU panggilan refresh", proven under REAL
// concurrency, not sequentially. A sequential test would stay green
// even if every apiFetch call started its own refresh (exactly the bug
// this file guards against) — same trap as crm_be's lead_number
// allocation test (#19).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { apiFetch } from "./api-client";
import { ApiError } from "./api-types";

const ORIGINAL_LOCATION = window.location;

// apiFetch redirects via window.location.href on unrecoverable auth
// failure. jsdom's real Location throws "not implemented: navigation"
// on an href assignment, so it's swapped for a minimal stub —
// Object.defineProperty (not a plain assignment) because
// window.location's setter type only accepts `string`, not a `Location`
// or a stub object.
function mockLocation() {
  Object.defineProperty(window, "location", {
    configurable: true,
    value: { href: "" } as unknown as Location,
  });
}

function restoreLocation() {
  Object.defineProperty(window, "location", {
    configurable: true,
    value: ORIGINAL_LOCATION,
  });
}

// deferred() gives the test full control over WHEN a mocked fetch call
// resolves — used to keep the refresh call "in flight" long enough for
// every concurrent 401 to reach the single-flight check before any of
// them could possibly see a settled/reset refreshPromise.
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

// Typed against `typeof fetch` so fetchMock.mock.calls[N] carries fetch's
// real [input, init?] tuple, without declaring unused parameter names on
// every call site.
function stubFetch(response: Response) {
  const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(response));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  mockLocation();
});

afterEach(() => {
  restoreLocation();
  vi.unstubAllGlobals();
  document.cookie = "csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT";
});

describe("refresh single-flight", () => {
  it("N concurrent 401s trigger exactly ONE call to /v1/auth/refresh, then each request is retried exactly once", async () => {
    const refreshGate = deferred<Response>();
    let refreshCallCount = 0;
    let retryCallCount = 0;

    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/v1/auth/refresh")) {
        refreshCallCount++;
        return refreshGate.promise;
      }
      if (url.endsWith("/v1/widgets")) {
        // First wave (before refresh resolves): always 401. Anything
        // after that is the post-refresh retry.
        retryCallCount++;
        if (retryCallCount > 6) {
          return Promise.resolve(jsonResponse(200, { data: { ok: true } }));
        }
        return Promise.resolve(jsonResponse(401, { error: { code: "authentication_required", message: "Token tidak valid." } }));
      }
      throw new Error(`unexpected fetch to ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    // Six "widgets" all firing at once — the exact scenario TD phase 3
    // §4.2 warns about.
    const calls = Array.from({ length: 6 }, () => apiFetch("/v1/widgets"));

    // Let every call's initial fetch (the 401) resolve and reach the
    // single-flight branch BEFORE the refresh call is allowed to
    // complete — this is what actually exercises concurrency instead
    // of accidentally testing a sequential path.
    await new Promise((r) => setTimeout(r, 0));
    await new Promise((r) => setTimeout(r, 0));

    expect(refreshCallCount).toBe(1);

    refreshGate.resolve(jsonResponse(200, { data: { status: "ok" } }));

    const results = await Promise.all(calls);

    expect(refreshCallCount).toBe(1);
    expect(results).toHaveLength(6);
    for (const result of results) {
      expect(result).toEqual({ ok: true });
    }
    // 6 initial 401s + 6 retries = 12 calls to /v1/widgets, 1 to refresh.
    expect(retryCallCount).toBe(12);
  });

  it("a retried request that gets 401 AGAIN does not trigger a second refresh", async () => {
    let refreshCallCount = 0;

    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/v1/auth/refresh")) {
        refreshCallCount++;
        return Promise.resolve(jsonResponse(200, { data: { status: "ok" } }));
      }
      if (url.endsWith("/v1/widgets")) {
        // ALWAYS 401 — refresh "succeeds" but the resource itself keeps
        // rejecting. The retry must still give up after one attempt.
        return Promise.resolve(jsonResponse(401, { error: { code: "authentication_required", message: "Token tidak valid." } }));
      }
      throw new Error(`unexpected fetch to ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiFetch("/v1/widgets")).rejects.toThrow(ApiError);

    expect(refreshCallCount).toBe(1);
  });

  it("a 401 from /v1/auth/refresh itself never triggers another refresh", async () => {
    let refreshCallCount = 0;

    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/v1/auth/refresh")) {
        refreshCallCount++;
        return Promise.resolve(jsonResponse(401, { error: { code: "authentication_required", message: "Sesi habis." } }));
      }
      if (url.endsWith("/v1/widgets")) {
        return Promise.resolve(jsonResponse(401, { error: { code: "authentication_required", message: "Token tidak valid." } }));
      }
      throw new Error(`unexpected fetch to ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiFetch("/v1/widgets")).rejects.toThrow(ApiError);

    expect(refreshCallCount).toBe(1);
    expect(window.location.href).toBe("/login");
  });
});

describe("CSRF header", () => {
  function setCsrfCookie(value: string) {
    document.cookie = `csrf_token=${value}; path=/`;
  }

  it("is attached on POST", async () => {
    setCsrfCookie("test-csrf-value");
    const fetchMock = stubFetch(jsonResponse(200, { data: { ok: true } }));

    await apiFetch("/v1/leads", { method: "POST", body: { name: "Budi" } });

    const [, init] = fetchMock.mock.calls[0];
    const headers = new Headers(init?.headers);
    expect(headers.get("X-CSRF-Token")).toBe("test-csrf-value");
  });

  it("is absent on GET", async () => {
    setCsrfCookie("test-csrf-value");
    const fetchMock = stubFetch(jsonResponse(200, { data: { ok: true } }));

    await apiFetch("/v1/leads");

    const [, init] = fetchMock.mock.calls[0];
    const headers = new Headers(init?.headers);
    expect(headers.has("X-CSRF-Token")).toBe(false);
  });
});

describe("error mapping", () => {
  it("throws ApiError carrying code/message/details from the error envelope", async () => {
    stubFetch(
      jsonResponse(400, {
        error: {
          code: "validation_failed",
          message: "Permintaan tidak valid.",
          details: [{ field: "email", code: "required" }],
        },
      })
    );

    await expect(apiFetch("/v1/leads", { method: "POST", body: {} })).rejects.toMatchObject({
      code: "validation_failed",
      status: 400,
      details: [{ field: "email", code: "required" }],
    });
  });
});
