/*
 * Jualin CRM — companion script for embedded forms (keputusan D8).
 * Served from the SAME origin as /embed/{public_key} (GET /embed.js),
 * cached publicly (unlike the embed page itself, this file carries no
 * per-form or per-request state — one file, same bytes, for every form).
 *
 * Progressive enhancement, not a requirement: the iframe this pairs
 * with (installed via the snippet's <iframe data-jualin-form ...>) is
 * fully visible and fully submittable without this script ever loading
 * — only its HEIGHT stays fixed at whatever the snippet's own
 * height="..." attribute says. A customer's site that blocks this
 * script, or a network hiccup that drops it, never loses the form
 * itself.
 *
 * Four constraints this file exists specifically to hold the line on
 * (docs/phases/06-connect-form/td.md §7 "Auto-resize"):
 *   1. Only ONE message type is ever recognized ("jualin:resize") —
 *      this must never become a general command channel from the
 *      iframe to the host page.
 *   2. e.origin is verified against EMBED_ORIGIN before anything else
 *      runs — without this, ANY page could postMessage a fake resize
 *      event here.
 *   3. Height is clamped to 100-4000px — an unclamped height lets a
 *      malicious message grow the iframe over the entire host page
 *      (reverse clickjacking).
 *   4. The iframe is matched by e.source (the actual window that sent
 *      the message), never just "the first iframe on the page" — a
 *      page with more than one embedded form must resize the right one.
 */
(function () {
  "use strict";

  var EMBED_ORIGIN = (function () {
    var s = document.currentScript;
    if (!s || !s.src) return null;
    try {
      return new URL(s.src).origin;
    } catch (e) {
      return null;
    }
  })();
  if (!EMBED_ORIGIN) return;

  window.addEventListener("message", function (e) {
    if (e.origin !== EMBED_ORIGIN) return; // constraint #2 — mandatory

    var d = e.data;
    if (!d || d.type !== "jualin:resize") return; // constraint #1 — single message type

    var h = Number(d.height);
    if (!isFinite(h) || h < 100 || h > 4000) return; // constraint #3 — clamped range

    var frames = document.querySelectorAll("iframe[data-jualin-form]");
    for (var i = 0; i < frames.length; i++) {
      if (frames[i].contentWindow === e.source) { // constraint #4 — matched by source, not position
        frames[i].style.height = h + "px";
        break;
      }
    }
  });
})();
