// Pure rules behind the Laporan screen (Phase 8.6 #171), kept out of the
// component so Vitest can reach them.
import type { EmployeeMetric } from "./metrics";

// Brief §10.5: "Data sedikit (mis. 3 lead) — angka tetap tampil, dengan
// catatan bahwa angkanya belum bermakna". Below ten leads a single won lead
// moves the conversion rate by ten points or more.
export const FEW_LEADS = 10;

export type DataVolume = "empty" | "few" | "enough";

export function dataVolume(totalNew: number): DataVolume {
  if (totalNew <= 0) return "empty";
  return totalNew < FEW_LEADS ? "few" : "enough";
}

export type MemberSortKey = "lead_count" | "avg_response_seconds" | "converted_count" | "converted_share";

export interface MemberRow extends EmployeeMetric {
  /** converted_count ÷ lead_count, null when the member holds no lead. This
   *  is "share of their leads converted" — NOT the organization's conversion
   *  rate, which excludes spam and unqualified from the denominator; the API
   *  gives no per-member status breakdown to do that. The screen labels it
   *  as its own thing so the two can't be read as the same number. */
  converted_share: number | null;
}

export function memberRows(employees: EmployeeMetric[], sort: MemberSortKey): MemberRow[] {
  const rows = employees.map((e) => ({
    ...e,
    converted_share: e.lead_count > 0 ? e.converted_count / e.lead_count : null,
  }));
  // Fastest response first; every other column biggest first. Missing values
  // (never touched, no leads) always sink to the bottom, whichever the sort.
  const ascending = sort === "avg_response_seconds";
  return rows.sort((a, b) => {
    const x = a[sort];
    const y = b[sort];
    if (x === null && y === null) return a.full_name.localeCompare(b.full_name);
    if (x === null) return 1;
    if (y === null) return -1;
    if (x === y) return a.full_name.localeCompare(b.full_name);
    return ascending ? x - y : y - x;
  });
}
