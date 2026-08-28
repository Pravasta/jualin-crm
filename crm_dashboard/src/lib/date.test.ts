import { describe, expect, it } from "vitest";
import { dateInputToEndOfDayUTC, dateInputToStartOfDayUTC, formatApproximateID, formatDateID } from "./date";

describe("dateInputToStartOfDayUTC / dateInputToEndOfDayUTC", () => {
  it("anchors the FROM bound to the start of the day and TO to the end", () => {
    // Without this, a lead created late on the last day of a
    // created_to=2026-08-31 range would be silently excluded — the
    // bound would land at 2026-08-31T00:00:00Z instead of end-of-day.
    expect(dateInputToStartOfDayUTC("2026-08-01")).toBe("2026-08-01T00:00:00.000Z");
    expect(dateInputToEndOfDayUTC("2026-08-31")).toBe("2026-08-31T23:59:59.999Z");
  });

  it("returns undefined for an empty input rather than an invalid timestamp", () => {
    expect(dateInputToStartOfDayUTC("")).toBeUndefined();
    expect(dateInputToEndOfDayUTC("")).toBeUndefined();
  });
});

describe("formatDateID", () => {
  it("formats as day-shortmonth-year Indonesian", () => {
    expect(formatDateID("2026-08-17T09:30:00Z")).toBe("17 Agu 2026");
  });
});

describe("formatApproximateID", () => {
  const NOW = new Date("2026-08-28T12:00:00Z");

  it("under a minute reads as just now, not \"0 menit lalu\"", () => {
    expect(formatApproximateID("2026-08-28T11:59:45Z", NOW)).toBe("sekitar baru saja");
  });

  it("minutes", () => {
    expect(formatApproximateID("2026-08-28T11:55:00Z", NOW)).toBe("sekitar 5 menit lalu");
  });

  it("hours", () => {
    expect(formatApproximateID("2026-08-28T09:00:00Z", NOW)).toBe("sekitar 3 jam lalu");
  });

  it("days", () => {
    expect(formatApproximateID("2026-08-25T12:00:00Z", NOW)).toBe("sekitar 3 hari lalu");
  });

  it("never formats a precise timestamp — always the approximate wording", () => {
    // last_used_at is throttled server-side (TD phase 4 §10); this is
    // the client-side guarantee that matches — no exact clock time ever
    // appears in the label.
    expect(formatApproximateID("2026-08-28T11:30:00Z", NOW)).not.toMatch(/\d{1,2}:\d{2}/);
  });
});
