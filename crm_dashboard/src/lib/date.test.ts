import { describe, expect, it } from "vitest";
import { dateInputToEndOfDayUTC, dateInputToStartOfDayUTC, formatDateID } from "./date";

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
