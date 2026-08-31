// Builds the embed snippet an Owner copies out of the form editor and
// pastes into their own website (#89). Split into lib/ and covered by
// form-snippet.test.ts because this is the one piece of #89 that can be
// silently wrong: a snippet that looks right but points at the wrong
// origin, drops the public_key, or leaves the form name unescaped in the
// title="" attribute produces a broken install the Owner only discovers
// as a missing lead (PRD §Catatan: "setiap langkah yang gagal dijelaskan
// datang sebagai tiket support").
//
// The embed page and embed.js are served by crm_be (GET /embed/{key},
// GET /embed.js — TD §7 keputusan D1), so the embed base is the backend
// origin today. It has its own env var rather than reusing the API base
// because ADR-005's D1 obligation is that the embed page MUST move to a
// different hostname from the dashboard when deployment happens — a
// separate var now means that move is a config change, not a code change.
//
// Read at call time, not module load, so a trailing-slash or missing
// value is handled here and tests can stub the env without import-order
// games.
export function embedBaseUrl(): string {
  // `||`, not `??` — an empty string (unset in some deploy setups) must
  // fall through to the next candidate, not be treated as a real value.
  const raw =
    process.env.NEXT_PUBLIC_EMBED_BASE_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "";
  return raw.replace(/\/+$/, "");
}

// The form name lands inside title="..." — it's customer-entered text
// (they typed it in the "Nama" field of the create dialog), so it has to
// be escaped for an HTML double-quoted attribute context or a name
// containing " or < breaks the tag. Mirrors what html/template does
// server-side for the embed page's own field labels (TD §16 risiko:
// "XSS lewat label field").
export function escapeHtmlAttribute(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export interface SnippetParams {
  publicKey: string;
  formName: string;
}

// height="620" is the initial value before embed.js has had a chance to
// run — without it the iframe flickers from the browser's default height
// to the real one (TD §10). It stays in the fixed-height variant too as
// a reasonable default for a form nobody is auto-resizing.
const INITIAL_HEIGHT = 620;

// Recommended variant — height follows content via the companion script.
export function autoResizeSnippet({ publicKey, formName }: SnippetParams): string {
  const base = embedBaseUrl();
  const title = escapeHtmlAttribute(formName);
  return (
    `<iframe src="${base}/embed/${publicKey}" data-jualin-form\n` +
    `        width="100%" height="${INITIAL_HEIGHT}" style="border:0" title="${title}"></iframe>\n` +
    `<script src="${base}/embed.js" async></script>`
  );
}

// No-script variant — for sites that forbid third-party <script>. Works
// identically; the height is just fixed (TD §10: "keduanya sama-sama
// bekerja; yang kedua hanya bertinggi tetap").
export function fixedHeightSnippet({ publicKey, formName }: SnippetParams): string {
  const base = embedBaseUrl();
  const title = escapeHtmlAttribute(formName);
  return (
    `<iframe src="${base}/embed/${publicKey}"\n` +
    `        width="100%" height="${INITIAL_HEIGHT}" style="border:0" title="${title}"></iframe>`
  );
}

// JSX variant — style must be an object and attributes camelCased, or
// React refuses to render it. Same auto-resize behaviour as
// autoResizeSnippet (TD §10's note about the JSX adjustment).
export function jsxSnippet({ publicKey, formName }: SnippetParams): string {
  const base = embedBaseUrl();
  // In JSX the title is a normal string literal, not an HTML attribute —
  // React escapes it at render time, so it is passed through as typed
  // rather than entity-encoded here.
  return (
    `<iframe\n` +
    `  src="${base}/embed/${publicKey}"\n` +
    `  data-jualin-form\n` +
    `  width="100%"\n` +
    `  height={${INITIAL_HEIGHT}}\n` +
    `  style={{ border: 0 }}\n` +
    `  title={${JSON.stringify(formName)}}\n` +
    `/>\n` +
    `<script src="${base}/embed.js" async></script>`
  );
}
