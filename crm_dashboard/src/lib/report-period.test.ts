import { describe, expect, it } from "vitest";
import { customRange, presetRange } from "./report-period";

// Local Date constructors, so these hold in any machine timezone.
const now = new Date(2026, 8, 25, 14, 30); // Jumat 25 Sep 2026, 14:30 local

describe("presetRange", () => {
  it("7 hari = today and the six days before, whole local days", () => {
    const r = presetRange("7d", now);
    expect([r.fromDate, r.toDate]).toEqual(["2026-09-19", "2026-09-25"]);
    expect(new Date(r.from).getTime()).toBe(new Date(2026, 8, 19, 0, 0, 0, 0).getTime());
    expect(new Date(r.to).getTime()).toBe(new Date(2026, 8, 25, 23, 59, 59, 999).getTime());
  });

  it("30 hari spans thirty calendar days", () => {
    const r = presetRange("30d", now);
    expect([r.fromDate, r.toDate]).toEqual(["2026-08-27", "2026-09-25"]);
  });

  it("bulan ini runs from the 1st to today", () => {
    expect(presetRange("month", now)).toMatchObject({ fromDate: "2026-09-01", toDate: "2026-09-25" });
  });

  it("bulan lalu is the whole previous month, across a year boundary too", () => {
    expect(presetRange("last_month", now)).toMatchObject({ fromDate: "2026-08-01", toDate: "2026-08-31" });
    expect(presetRange("last_month", new Date(2026, 0, 10))).toMatchObject({
      fromDate: "2025-12-01",
      toDate: "2025-12-31",
    });
  });
});

describe("customRange", () => {
  it("accepts a valid range as whole local days", () => {
    const r = customRange("2026-09-01", "2026-09-15");
    expect("range" in r && [r.range.fromDate, r.range.toDate]).toEqual(["2026-09-01", "2026-09-15"]);
  });

  it("names the problem instead of letting the API 400", () => {
    expect(customRange("", "2026-09-15")).toEqual({ problem: "Isi tanggal mulai dan tanggal akhir." });
    expect(customRange("2026-09-15", "2026-09-01")).toEqual({
      problem: "Tanggal akhir tidak boleh sebelum tanggal mulai.",
    });
    expect(customRange("2026-02-31", "2026-03-01")).toEqual({ problem: "Isi tanggal mulai dan tanggal akhir." });
  });

  it("caps at 366 days, the API's own limit", () => {
    expect("range" in customRange("2025-01-01", "2026-01-01")).toBe(true); // 366 days
    expect(customRange("2025-01-01", "2026-01-02")).toEqual({ problem: "Rentang paling panjang 366 hari." });
  });
});
