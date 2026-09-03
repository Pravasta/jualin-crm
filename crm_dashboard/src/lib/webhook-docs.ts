// The signature-verification examples the /connect/webhook/docs page
// renders (issue #104, Phase 7 close). One builder per language so the
// three can never quietly drift into different constructions of the
// signed payload — the bug that would make every receiver reject every
// delivery while our own tests stay green (signature_test.go's own
// warning: a sign-then-verify test shares the mistake and passes).
//
// Unlike buildCurlExample (#49), these examples are GENUINELY runnable as
// printed. The curl example there needs a real secret embedded to call
// us, which no durable page can hold (Aturan #21). A verification example
// needs no secret of ours: the receiver pastes the secret THEY saved from
// the create dialog into WEBHOOK_SIGNING_SECRET and the code works. That
// is what makes acceptance criterion #2 ("contoh disalin, dijalankan,
// benar-benar memvalidasi — lalu payload diubah satu byte dan ditolak")
// honest rather than aspirational.
//
// The construction every example encodes, from crm_be/internal/webhook/
// signature.go:
//
//   header  = "X-Jualin-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>"
//   signed  = "<t>" + "." + "<raw request body, exact bytes>"
//   v1      = hex( HMAC-SHA256(secret, signed) )
//
// Two details each example gets right because getting them wrong is the
// common failure:
//
//   - the HMAC is over the RAW body, before any JSON parse/re-serialise —
//     re-encoding reorders keys and breaks the signature with no error on
//     either side;
//   - `t` is INSIDE the signed string, and the 5-minute tolerance is the
//     receiver's to enforce (we sign a retry with the time it is actually
//     sent, so a retry hours later is still within its own window).

/** The header crm_be sends the signature in. */
export const SIGNATURE_HEADER = "X-Jualin-Signature";

/** Replay tolerance the docs tell receivers to enforce (td.md §2, D4). */
export const SIGNATURE_TOLERANCE_SECONDS = 300;

/** Placeholder the reader replaces with the secret they saved on create. */
export const SECRET_PLACEHOLDER = "whsec_...";

export const NODE_EXAMPLE = `// Node.js / Express — the raw body is required, so express.json() is NOT
// used on this route: it would parse and discard the exact bytes the
// signature covers.
const express = require("express");
const crypto = require("crypto");

const SIGNING_SECRET = process.env.WEBHOOK_SIGNING_SECRET; // "${SECRET_PLACEHOLDER}"
const TOLERANCE_SECONDS = ${SIGNATURE_TOLERANCE_SECONDS};

const app = express();

app.post(
  "/webhook/jualin",
  express.raw({ type: "*/*" }), // req.body is a Buffer of the exact bytes
  (req, res) => {
    const header = req.get("${SIGNATURE_HEADER}") || "";
    const parts = Object.fromEntries(
      header.split(",").map((kv) => kv.split("=").map((s) => s.trim()))
    );
    const timestamp = parts.t;
    const received = parts.v1;
    if (!timestamp || !received) return res.status(400).send("missing signature");

    // Reject stale deliveries — the timestamp is signed, so it cannot be
    // moved without breaking v1 below.
    const age = Math.abs(Date.now() / 1000 - Number(timestamp));
    if (age > TOLERANCE_SECONDS) return res.status(400).send("timestamp outside tolerance");

    const signedPayload = timestamp + "." + req.body.toString("utf8");
    const expected = crypto
      .createHmac("sha256", SIGNING_SECRET)
      .update(signedPayload)
      .digest("hex");

    const ok =
      received.length === expected.length &&
      crypto.timingSafeEqual(Buffer.from(received), Buffer.from(expected));
    if (!ok) return res.status(400).send("invalid signature");

    const event = JSON.parse(req.body.toString("utf8"));
    // event.delivery_id is stable across retries — use it to de-duplicate
    // (deliveries are at-least-once).
    console.log("verified", event.event, event.delivery_id);
    res.sendStatus(200);
  }
);

app.listen(9099);`;

export const PYTHON_EXAMPLE = `# Python / Flask — request.get_data() returns the raw bytes; do not use
# request.json before verifying (it would parse the bytes the signature
# covers).
import hashlib
import hmac
import os
import time

from flask import Flask, request

SIGNING_SECRET = os.environ["WEBHOOK_SIGNING_SECRET"]  # "${SECRET_PLACEHOLDER}"
TOLERANCE_SECONDS = ${SIGNATURE_TOLERANCE_SECONDS}

app = Flask(__name__)


@app.post("/webhook/jualin")
def jualin_webhook():
    header = request.headers.get("${SIGNATURE_HEADER}", "")
    parts = dict(p.strip().split("=", 1) for p in header.split(",") if "=" in p)
    timestamp = parts.get("t")
    received = parts.get("v1")
    if not timestamp or not received:
        return "missing signature", 400

    # The timestamp is inside the signed payload, so a replayed request
    # cannot have its t rewritten without invalidating v1.
    if abs(time.time() - int(timestamp)) > TOLERANCE_SECONDS:
        return "timestamp outside tolerance", 400

    raw_body = request.get_data()  # bytes, exactly as received
    signed_payload = timestamp.encode() + b"." + raw_body
    expected = hmac.new(
        SIGNING_SECRET.encode(), signed_payload, hashlib.sha256
    ).hexdigest()

    if not hmac.compare_digest(received, expected):
        return "invalid signature", 400

    event = request.get_json()
    # event["delivery_id"] is stable across retries — de-duplicate on it.
    app.logger.info("verified %s %s", event["event"], event["delivery_id"])
    return "", 200`;

export const PHP_EXAMPLE = `<?php
// Plain PHP — php://input is the raw request body. Never json_decode
// before verifying: re-encoding the parsed value changes the bytes the
// signature covers.

$signingSecret = getenv("WEBHOOK_SIGNING_SECRET"); // "${SECRET_PLACEHOLDER}"
$toleranceSeconds = ${SIGNATURE_TOLERANCE_SECONDS};

$header = $_SERVER["HTTP_X_JUALIN_SIGNATURE"] ?? "";
$parts = [];
foreach (explode(",", $header) as $kv) {
    $pair = explode("=", trim($kv), 2);
    if (count($pair) === 2) {
        $parts[$pair[0]] = $pair[1];
    }
}
$timestamp = $parts["t"] ?? null;
$received = $parts["v1"] ?? null;
if ($timestamp === null || $received === null) {
    http_response_code(400);
    exit("missing signature");
}

// Signed timestamp — cannot be moved without breaking v1.
if (abs(time() - (int) $timestamp) > $toleranceSeconds) {
    http_response_code(400);
    exit("timestamp outside tolerance");
}

$rawBody = file_get_contents("php://input");
$signedPayload = $timestamp . "." . $rawBody;
$expected = hash_hmac("sha256", $signedPayload, $signingSecret);

if (!hash_equals($expected, $received)) {
    http_response_code(400);
    exit("invalid signature");
}

$event = json_decode($rawBody, true);
// $event["delivery_id"] is stable across retries — de-duplicate on it.
error_log("verified " . $event["event"] . " " . $event["delivery_id"]);
http_response_code(200);`;

export interface WebhookDocExample {
  language: string;
  /** Filename hint shown above the block, e.g. "server.js". */
  filename: string;
  code: string;
}

// Order: Node, PHP, Python — the same order td.md §2 and the issue name
// them, so a reader following the issue finds them where expected.
export const WEBHOOK_DOC_EXAMPLES: readonly WebhookDocExample[] = [
  { language: "Node.js", filename: "server.js", code: NODE_EXAMPLE },
  { language: "PHP", filename: "webhook.php", code: PHP_EXAMPLE },
  { language: "Python", filename: "app.py", code: PYTHON_EXAMPLE },
] as const;

// One sample delivery body, rendered in the "bentuk payload" section. Not
// fetched from the API — it is the frozen shape from
// crm_be/internal/lead/webhook_payload.go plus the delivery_id the worker
// splices in (delivery_id.go), shown so a receiver knows the field order
// and types before the first real delivery arrives.
export const SAMPLE_PAYLOAD = `{
  "delivery_id": "0192f0a1-8c7e-7c2a-9f3b-2a1e4d6c8b00",
  "event": "lead.created",
  "occurred_at": "2026-09-03T04:21:07.531933Z",
  "organization_id": "0192e0b3-1a2c-7d4e-8f5a-6b7c8d9e0f11",
  "data": {
    "lead": {
      "id": "0192f0a1-7b6d-7e1c-8a2f-3c4d5e6f7a80",
      "lead_number": 42,
      "name": "Budi Santoso",
      "email": "budi@example.com",
      "phone": "0812-3456-7890",
      "phone_e164": "+6281234567890",
      "company": "Toko Budi",
      "notes": "",
      "status": "new",
      "lost_reason": null,
      "source": "manual",
      "source_api_key_id": null,
      "source_form_id": null,
      "assigned_to_membership_id": null,
      "version": 1,
      "created_by_membership_id": "0192e0b3-9f8e-7d6c-8b5a-4c3d2e1f0a99",
      "created_at": "2026-09-03T04:21:07.512088Z",
      "updated_at": "2026-09-03T04:21:07.512088Z"
    }
  }
}`;

// The lead.status_changed body adds one key. Shown as a diff-in-prose
// rather than a second full dump so the reader sees exactly what is
// different, not two 20-line blocks to compare by eye.
export const SAMPLE_STATUS_CHANGED_EXTRA = `{
  "event": "lead.status_changed",
  "changes": {
    "status": { "from": "new", "to": "contacted" }
  },
  "data": { "lead": { "...": "same shape, status now \\"contacted\\"" } }
}`;
