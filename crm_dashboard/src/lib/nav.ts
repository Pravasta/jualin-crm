// Navigation metadata and the pure logic around it. Split out of
// app-shell.tsx so the parts that can be silently wrong — which item is
// active, which title shows — are testable without rendering React.

export interface NavItemConfig {
  href: string;
  label: string;
  /** Optional count shown as a pill. Wired to unassigned leads in #32. */
  badge?: number;
}

// "Lead" and "Customer" stay as-is — glossary.md fixes both as the
// product's own terms. "Home"/"Task"/"Settings" from the design are
// translated: those have ordinary Indonesian words, and acceptance
// criterion #12 requires the interface to be Indonesian throughout.
// "Connect" (between Tim and Pengaturan) is issue #86/ADR-012 — the
// capture-channel surface (API, Formulir, Webhook) raised out of
// Pengaturan into its own menu item. Visible to EVERY role (keputusan
// D6): this list is never filtered by role, the same way /settings
// isn't today — the gate for what a role can actually do inside
// Connect lives in the screens themselves (canManageAPIKeys today,
// canManageForms in #89), not here.
// "Langganan" (#125) is visible to EVERY role, same as "Connect" above —
// what a role can actually do inside it (Owner+Admin view, Owner-only
// change) is enforced by the screen itself
// (lib/subscription-permissions.ts) and by crm_be's authz, not by
// filtering this list.
export const NAV_ITEMS: NavItemConfig[] = [
  { href: "/", label: "Beranda" },
  { href: "/leads", label: "Lead" },
  { href: "/customers", label: "Customer" },
  { href: "/tasks", label: "Tugas" },
  { href: "/team", label: "Tim" },
  { href: "/connect", label: "Connect" },
  { href: "/subscription", label: "Langganan" },
  { href: "/settings", label: "Pengaturan" },
];

// Exact match for "/" so it isn't active on every page; prefix match for
// the rest so /leads/{id} keeps "Lead" highlighted on a detail screen.
export function isActive(pathname: string, href: string): boolean {
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}

// Longest href first: without it, "/" would win for every path and every
// page would be titled "Beranda".
export function pageTitle(pathname: string): string {
  const match = [...NAV_ITEMS]
    .sort((a, b) => b.href.length - a.href.length)
    .find((item) => isActive(pathname, item.href));
  return match?.label ?? "Jualin CRM";
}

export function initialsOf(fullName: string): string {
  const parts = fullName.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}
