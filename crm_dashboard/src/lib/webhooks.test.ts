import { describe, expect, it } from "vitest";
import {
  canCloseCreateDialog,
  WEBHOOK_EVENTS,
  WEBHOOK_EVENT_DESCRIPTIONS,
  WEBHOOK_EVENT_LABELS,
  type CreatedWebhookEndpoint,
  type WebhookEndpoint,
  type WebhookEvent,
} from "./webhooks";

describe("WEBHOOK_EVENTS", () => {
  // The union has to stay in step with crm_be's webhook.KnownEvents.
  // Nothing at compile time can reach across the two languages, so this
  // pins the literal strings the backend validates against — a rename on
  // either side shows up here rather than as a subscription that silently
  // never fires.
  it("matches the backend's KnownEvents exactly", () => {
    expect([...WEBHOOK_EVENTS]).toEqual(["lead.created", "lead.status_changed"]);
  });

  it("has a label and a description for every event", () => {
    for (const event of WEBHOOK_EVENTS) {
      expect(WEBHOOK_EVENT_LABELS[event]).toBeTruthy();
      expect(WEBHOOK_EVENT_DESCRIPTIONS[event]).toBeTruthy();
    }
  });

  // A description that only restates the event name teaches nothing to
  // the person choosing which to subscribe to.
  it("describes when each event fires rather than repeating its name", () => {
    for (const event of WEBHOOK_EVENTS) {
      expect(WEBHOOK_EVENT_DESCRIPTIONS[event]).not.toContain(event);
    }
  });
});

// These two tests assert a TYPE guarantee, which is why they read as
// trivial at runtime: the real check happens in `npm run typecheck`.
// Assigning a secret onto a WebhookEndpoint below would fail the build,
// which is the whole point — Aturan #21 enforced by the compiler rather
// than by remembering not to render a field (the CreatedAPIKey precedent
// from #48).
describe("secret is reachable only from create", () => {
  it("is present on CreatedWebhookEndpoint", () => {
    const created: CreatedWebhookEndpoint = {
      id: "01a0",
      url: "https://receiver.example.com/hook",
      secret_prefix: "whsec_abcd1234",
      events: ["lead.created"],
      description: "",
      is_active: true,
      created_by_membership_id: null,
      created_at: "2026-09-02T10:00:00Z",
      updated_at: "2026-09-02T10:00:00Z",
      secret: "whsec_the-only-time-this-exists",
    };
    expect(created.secret).toBeTruthy();
  });

  it("is absent from the list type", () => {
    const listed: WebhookEndpoint = {
      id: "01a0",
      url: "https://receiver.example.com/hook",
      secret_prefix: "whsec_abcd1234",
      events: ["lead.created"],
      description: "",
      is_active: true,
      created_by_membership_id: null,
      created_at: "2026-09-02T10:00:00Z",
      updated_at: "2026-09-02T10:00:00Z",
    };
    // @ts-expect-error — WebhookEndpoint must never carry a secret. If
    // this line ever stops erroring, the type has grown a field that
    // would let the list screen render a credential.
    expect(listed.secret).toBeUndefined();
  });

  // secret_prefix is safe to show, and must stay clearly distinct from
  // the secret itself: it is 8 characters of a 49-character value.
  it("keeps secret_prefix shorter than a usable secret", () => {
    const prefix = "whsec_abcd1234";
    expect(prefix.length).toBeLessThan("whsec_".length + 43);
  });
});

describe("WebhookEvent", () => {
  it("narrows to the known literals", () => {
    const event: WebhookEvent = "lead.created";
    expect(WEBHOOK_EVENTS).toContain(event);
  });
});

// The reveal step is the only place in the product where a credential is
// visible and unrecoverable. These cases are the close guard — the reason
// they are testable at all is that the logic was pulled out of the
// component (TD phase 3 §9 keeps visual testing out of scope, so a rule
// left inline in JSX would have had no proof at all).
describe("canCloseCreateDialog", () => {
  it("allows closing freely while still filling the form", () => {
    expect(canCloseCreateDialog("form", false)).toBe(true);
    expect(canCloseCreateDialog("form", true)).toBe(true);
  });

  // The whole point: X, Escape, and a backdrop click all route through
  // this one answer, so a false here blocks every one of them.
  it("blocks closing while the secret is shown and unacknowledged", () => {
    expect(canCloseCreateDialog("reveal", false)).toBe(false);
  });

  it("allows closing once the user confirms they saved it", () => {
    expect(canCloseCreateDialog("reveal", true)).toBe(true);
  });
});
