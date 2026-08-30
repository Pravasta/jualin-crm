"use client";

// The frame every protected screen renders inside — sidebar, page title,
// and the logout control. Built from the Claude Design output (issue
// #40); the screens themselves land in #32–#35.
//
// Which item is active and which title shows lives in @/lib/nav, so that
// logic is unit-tested without rendering React.
import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Home,
  Users,
  UserRound,
  SquareCheckBig,
  UsersRound,
  Plug,
  Settings,
  LogOut,
  type LucideIcon,
} from "lucide-react";

import { logout } from "@/lib/auth";
import { ROLE_LABELS, type Role } from "@/lib/labels";
import { getMetricsSummary } from "@/lib/metrics";
import { initialsOf, isActive, NAV_ITEMS, pageTitle } from "@/lib/nav";
import { useSession } from "@/lib/session-context";
import { cn } from "@/lib/utils";
import { NotificationBell } from "@/components/notification-bell";

// Icons live here rather than in nav.ts to keep that module free of
// React/JSX imports — it stays plain data plus pure functions.
const NAV_ICONS: Record<string, LucideIcon> = {
  "/": Home,
  "/leads": Users,
  "/customers": UserRound,
  "/tasks": SquareCheckBig,
  "/team": UsersRound,
  "/connect": Plug,
  "/settings": Settings,
};

export function AppShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const session = useSession();

  // All-time unassigned count (no period filter) — shown regardless of
  // which page is open, since it's a safety-net signal (freeze 2.3
  // ketentuan #3), not something scoped to whatever the lead list's own
  // filters currently are.
  const [unassignedCount, setUnassignedCount] = useState<number | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    getMetricsSummary({}, controller.signal)
      .then((s) => setUnassignedCount(s.unassigned))
      .catch(() => {});
    return () => controller.abort();
  }, []);

  async function handleLogout() {
    // logout always succeeds from the client's side — crm_be answers 204
    // unconditionally (internal/auth's not-found-is-success reasoning).
    await logout().catch(() => {});
    router.push("/login");
  }

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 flex h-screen w-58 shrink-0 flex-col border-r border-border bg-background">
        <div className="flex items-center gap-2.5 px-4.5 pt-4.5 pb-3.5">
          <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-linear-to-br from-[oklch(0.72_0.17_55)] to-[oklch(0.52_0.19_35)] text-sm font-bold text-white">
            J
          </div>
          <span className="text-[15px] font-semibold tracking-tight">Jualin CRM</span>
        </div>

        <div className="mb-1.5 border-b border-border px-4.5 pb-3.5 text-xs text-muted-foreground">
          {session.organization_name}
        </div>

        <nav className="flex flex-1 flex-col gap-px overflow-y-auto p-2.5">
          {NAV_ITEMS.map((item) => {
            const active = isActive(pathname, item.href);
            const Icon = NAV_ICONS[item.href];
            const badge = item.href === "/leads" ? (unassignedCount ?? undefined) : item.badge;
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13.5px] transition-colors",
                  active
                    ? "bg-accent-tint font-semibold text-accent-strong"
                    : "font-medium text-foreground/80 hover:bg-muted"
                )}
              >
                {Icon ? <Icon className="size-4 shrink-0" aria-hidden /> : null}
                <span className="flex-1">{item.label}</span>
                {badge ? (
                  <span className="min-w-4 rounded-full bg-accent-tint px-1.5 text-center text-[10.5px] font-semibold text-accent-strong">
                    {badge}
                  </span>
                ) : null}
              </Link>
            );
          })}
        </nav>

        <div className="border-t border-border p-3">
          <div className="flex items-center gap-2.5 px-2.5 py-1.5">
            <div className="flex size-6.5 shrink-0 items-center justify-center rounded-full bg-accent-tint text-[11.5px] font-semibold text-accent-strong">
              {initialsOf(session.full_name)}
            </div>
            <div className="min-w-0 flex-1">
              <div className="truncate text-[13px] font-medium">{session.full_name}</div>
              <div className="text-[11.5px] text-muted-foreground">
                {ROLE_LABELS[session.role as Role]}
              </div>
            </div>
            <button
              type="button"
              onClick={handleLogout}
              title="Keluar"
              aria-label="Keluar"
              className="flex cursor-pointer rounded-sm p-1 text-muted-foreground transition-colors hover:text-foreground"
            >
              <LogOut className="size-4" aria-hidden />
            </button>
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center justify-between border-b border-border bg-background px-6">
          <h1 className="text-[15px] font-semibold">{pageTitle(pathname)}</h1>
          <NotificationBell />
        </header>

        <main className="flex-1 bg-muted/30 p-6">{children}</main>
      </div>
    </div>
  );
}
