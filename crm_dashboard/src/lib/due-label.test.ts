import { describe, expect, it } from "vitest";
import { dueLabel, isOverdue } from "./due-label";

// Same cases as crm_employee/test/shared/due_label_test.dart (#153), so the
// dashboard (#165) and the phone read a task's due date the same way. Local
// Date constructors (no Z), so the tests don't depend on the machine's zone.
const now = new Date(2026, 8, 21, 14, 0); // Senin 21 Sep 2026, 14:00

describe("dueLabel — upcoming", () => {
  it("later today", () => {
    expect(dueLabel(new Date(2026, 8, 21, 17, 0), now)).toBe("Jatuh tempo hari ini");
  });
  it("tomorrow", () => {
    expect(dueLabel(new Date(2026, 8, 22, 9, 0), now)).toBe("Jatuh tempo besok");
  });
  it("a few days out", () => {
    expect(dueLabel(new Date(2026, 8, 24), now)).toBe("Jatuh tempo dalam 3 hari");
    expect(dueLabel(new Date(2026, 8, 28), now)).toBe("Jatuh tempo dalam 7 hari");
  });
  it("beyond a week: an absolute date, month in Indonesian", () => {
    expect(dueLabel(new Date(2026, 9, 12), now)).toBe("Jatuh tempo 12 Okt");
    expect(dueLabel(new Date(2026, 11, 3), now)).toBe("Jatuh tempo 3 Des");
  });
  it("another year: the year is shown too", () => {
    expect(dueLabel(new Date(2027, 0, 5), now)).toBe("Jatuh tempo 5 Jan 2027");
  });
});

describe("dueLabel — overdue", () => {
  it("earlier today, already passed", () => {
    expect(dueLabel(new Date(2026, 8, 21, 9, 0), now)).toBe("Terlambat");
  });
  it("yesterday and older", () => {
    expect(dueLabel(new Date(2026, 8, 20, 18, 0), now)).toBe("Terlambat 1 hari");
    expect(dueLabel(new Date(2026, 8, 14), now)).toBe("Terlambat 7 hari");
  });
});

describe("dueLabel — midnight boundary: calendar days, not 24-hour blocks", () => {
  it("late tonight, due early tomorrow morning: besok", () => {
    expect(dueLabel(new Date(2026, 8, 22, 8, 0), new Date(2026, 8, 21, 23, 30))).toBe("Jatuh tempo besok");
  });
  it("just after midnight, due later the same day: hari ini", () => {
    expect(dueLabel(new Date(2026, 8, 22, 23, 0), new Date(2026, 8, 22, 0, 30))).toBe("Jatuh tempo hari ini");
  });
  it("just after midnight, due late last night: Terlambat 1 hari", () => {
    expect(dueLabel(new Date(2026, 8, 21, 23, 0), new Date(2026, 8, 22, 0, 30))).toBe("Terlambat 1 hari");
  });
});

it('says "Terlambat" exactly when isOverdue does — words and color agree', () => {
  for (const minutes of [-3000, -61, -1, 1, 61, 3000]) {
    const due = new Date(now.getTime() + minutes * 60_000);
    expect(dueLabel(due, now).startsWith("Terlambat")).toBe(isOverdue(due, now));
  }
});
