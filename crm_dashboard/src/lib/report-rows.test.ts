import { describe, expect, it } from "vitest";
import { dataVolume, memberRows } from "./report-rows";
import type { EmployeeMetric } from "./metrics";

const member = (full_name: string, lead_count: number, converted_count: number, avg: number | null): EmployeeMetric => ({
  membership_id: full_name,
  full_name,
  lead_count,
  converted_count,
  avg_response_seconds: avg,
});

describe("dataVolume", () => {
  it("tells no data, a few leads, and enough apart", () => {
    expect(dataVolume(0)).toBe("empty");
    expect(dataVolume(3)).toBe("few");
    expect(dataVolume(9)).toBe("few");
    expect(dataVolume(10)).toBe("enough");
  });
});

describe("memberRows", () => {
  const data = [member("Budi", 10, 3, 2280), member("Sari", 18, 4, 4320), member("Andi", 24, 2, 9600), member("Rina", 0, 0, null)];

  it("derives the converted share per member, null without leads", () => {
    const rows = memberRows(data, "lead_count");
    expect(rows.find((r) => r.full_name === "Budi")?.converted_share).toBeCloseTo(0.3);
    expect(rows.find((r) => r.full_name === "Rina")?.converted_share).toBeNull();
  });

  it("sorts counts biggest first", () => {
    expect(memberRows(data, "lead_count").map((r) => r.full_name)).toEqual(["Andi", "Sari", "Budi", "Rina"]);
    expect(memberRows(data, "converted_share").map((r) => r.full_name)).toEqual(["Budi", "Sari", "Andi", "Rina"]);
  });

  it("sorts response time fastest first, and never-touched sinks either way", () => {
    expect(memberRows(data, "avg_response_seconds").map((r) => r.full_name)).toEqual(["Budi", "Sari", "Andi", "Rina"]);
  });
});
