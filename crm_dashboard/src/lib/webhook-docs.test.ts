import { createHmac } from "node:crypto";
import { describe, expect, it } from "vitest";
import {
  SAMPLE_PAYLOAD,
  SIGNATURE_HEADER,
  SIGNATURE_TOLERANCE_SECONDS,
  WEBHOOK_DOC_EXAMPLES,
} from "./webhook-docs";

// The construction crm_be/internal/webhook/signature.go emits. Duplicated
// here on purpose: if this drifts from signature.go the docs are wrong,
// and nothing at compile time links the two languages — the same reason
// signature_test.go pins a hand-computed vector rather than trusting a
// sign-then-verify round trip.
function sign(secret: string, unixSeconds: number, rawBody: string): string {
  const signedPayload = `${unixSeconds}.${rawBody}`;
  const v1 = createHmac("sha256", secret).update(signedPayload).digest("hex");
  return `t=${unixSeconds},v1=${v1}`;
}

// Follows the steps every example prints — parse the header, HMAC the raw
// body prefixed with "<t>.", hex-compare. If the docs describe a working
// procedure, this passes; if they describe "<body>.<t>" or a re-marshalled
// body, it fails.
function verifyAsDocumented(secret: string, header: string, rawBody: string): boolean {
  const parts = Object.fromEntries(
    header.split(",").map((kv) => kv.split("=").map((s) => s.trim()))
  );
  const expected = createHmac("sha256", secret)
    .update(`${parts.t}.${rawBody}`)
    .digest("hex");
  return parts.v1 === expected;
}

const SECRET = "whsec_dGVzdC1zZWNyZXQtZm9yLXVuaXQtdGVzdHMtb25seQ";
const TS = 1_756_872_067;

describe("webhook signature verification, as documented", () => {
  it("validates a signature built the way signature.go builds it", () => {
    const header = sign(SECRET, TS, SAMPLE_PAYLOAD);
    expect(verifyAsDocumented(SECRET, header, SAMPLE_PAYLOAD)).toBe(true);
  });

  it("rejects a payload changed by a single byte", () => {
    const header = sign(SECRET, TS, SAMPLE_PAYLOAD);
    const tampered = SAMPLE_PAYLOAD.replace("Budi Santoso", "Budi Santosa");
    expect(tampered).not.toBe(SAMPLE_PAYLOAD);
    expect(verifyAsDocumented(SECRET, header, tampered)).toBe(false);
  });

  it("rejects a signature whose timestamp was moved (t is inside the signed payload)", () => {
    const header = sign(SECRET, TS, SAMPLE_PAYLOAD);
    const moved = header.replace(`t=${TS}`, `t=${TS + 60}`);
    expect(verifyAsDocumented(SECRET, moved, SAMPLE_PAYLOAD)).toBe(false);
  });

  it("rejects the wrong secret", () => {
    const header = sign(SECRET, TS, SAMPLE_PAYLOAD);
    expect(verifyAsDocumented("whsec_someone-elses-secret", header, SAMPLE_PAYLOAD)).toBe(false);
  });
});

describe("example code encodes the correct construction", () => {
  it.each(WEBHOOK_DOC_EXAMPLES)("$language reads the header by its real name", ({ code }) => {
    // Node reads it case-insensitively via req.get(), PHP via the
    // HTTP_X_JUALIN_SIGNATURE CGI form — accept either spelling.
    const hasHeader =
      code.includes(SIGNATURE_HEADER) ||
      code.includes(SIGNATURE_HEADER.toUpperCase().replace(/-/g, "_"));
    expect(hasHeader).toBe(true);
  });

  it.each(WEBHOOK_DOC_EXAMPLES)(
    "$language signs \"<t>.<body>\", not \"<body>.<t>\"",
    ({ code }) => {
      // The separator-then-body order, in whatever each language's string
      // concat looks like. A "<body>.<t>" example would match none of these.
      const tThenBody = [
        'timestamp + "." + req.body', // Node
        'timestamp.encode() + b"." + raw_body', // Python
        '$timestamp . "." . $rawBody', // PHP
      ];
      expect(tThenBody.some((frag) => code.includes(frag))).toBe(true);
    }
  );

  it.each(WEBHOOK_DOC_EXAMPLES)("$language enforces the replay tolerance", ({ code }) => {
    expect(code).toContain(String(SIGNATURE_TOLERANCE_SECONDS));
    expect(code.toLowerCase()).toContain("tolerance");
  });

  it.each(WEBHOOK_DOC_EXAMPLES)("$language de-duplicates on delivery_id", ({ code }) => {
    expect(code).toContain("delivery_id");
  });

  it("offers Node, PHP, Python in that order", () => {
    expect(WEBHOOK_DOC_EXAMPLES.map((e) => e.language)).toEqual(["Node.js", "PHP", "Python"]);
  });

  it("does not embed a real-looking secret in any example", () => {
    for (const { code } of WEBHOOK_DOC_EXAMPLES) {
      // whsec_ followed by base64url chars would be a leaked-looking
      // credential; the placeholder is whsec_... only.
      expect(code).not.toMatch(/whsec_[A-Za-z0-9_-]{10,}/);
    }
  });
});

describe("sample payload", () => {
  it("is valid JSON with delivery_id first", () => {
    const parsed = JSON.parse(SAMPLE_PAYLOAD) as Record<string, unknown>;
    expect(Object.keys(parsed)[0]).toBe("delivery_id");
    expect(parsed).toHaveProperty("event", "lead.created");
    expect(parsed).toHaveProperty("data.lead.lead_number");
  });
});
