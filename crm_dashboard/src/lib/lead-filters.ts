// Pure logic for the lead list's URL-as-filter-state (issue #32
// acceptance criterion: filters reflected in the URL, reload/share-safe)
// — split out of leads-list.tsx so it's testable without rendering React,
// same pattern @/lib/nav used in #40.

export function parseCSVParam(value: string | null): string[] {
  return value ? value.split(",").filter(Boolean) : [];
}

export function toggleCSVValue(current: string[], value: string): string[] {
  return current.includes(value) ? current.filter((v) => v !== value) : [...current, value];
}

export interface LeadFilterState {
  status: string[];
  source: string[];
  assignedTo: string;
  keyword: string;
  createdFrom: string;
  createdTo: string;
}

export function hasAnyLeadFilter(filter: LeadFilterState): boolean {
  return (
    filter.status.length > 0 ||
    filter.source.length > 0 ||
    filter.assignedTo !== "" ||
    filter.keyword !== "" ||
    filter.createdFrom !== "" ||
    filter.createdTo !== ""
  );
}
