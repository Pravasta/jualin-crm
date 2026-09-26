"use client";

// The frame every protected screen renders inside. Built from the Claude
// Design output (issue #40), made responsive in #160 (td.md §4.1):
//
//   ≥1024px   sidebar 232px, full labels
//   768–1023  sidebar 68px, icons only (label in `title` + aria-label)
//   <768      no sidebar — top header + fixed bottom bar + "Lainnya" sheet
//
// Breakpoints are CSS (Tailwind md: = 768, lg: = 1024), never
// window.innerWidth as the prototype did: measuring width in JavaScript
// renders the desktop layout first and then jumps, and the server has no
// window to measure. Every layout below is present in the first HTML.
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
  CreditCard,
  Settings,
  LogOut,
  Ellipsis,
  ChartColumn,
  type LucideIcon,
} from "lucide-react";

import { logout } from "@/lib/auth";
import { ROLE_LABELS, type Role } from "@/lib/labels";
import { getMetricsSummary } from "@/lib/metrics";
import {
  BOTTOM_NAV_HREFS,
  initialsOf,
  isActive,
  isMoreActive,
  MORE_NAV_ITEMS,
  NAV_ITEMS,
  pageTitle,
} from "@/lib/nav";
import { useSession } from "@/lib/session-context";
import { cn } from "@/lib/utils";
import { NotificationBell } from "@/components/notification-bell";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";

// Icons live here rather than in nav.ts to keep that module free of
// React/JSX imports — it stays plain data plus pure functions.
const NAV_ICONS: Record<string, LucideIcon> = {
  "/": Home,
  "/leads": Users,
  "/customers": UserRound,
  "/tasks": SquareCheckBig,
  "/reports": ChartColumn,
  "/team": UsersRound,
  "/connect": Plug,
  "/subscription": CreditCard,
  "/settings": Settings,
};

const BOTTOM_NAV_ITEMS = NAV_ITEMS.filter((item) =>
  (BOTTOM_NAV_HREFS as readonly string[]).includes(item.href)
);

function BrandMark() {
  return (
    <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary text-sm font-bold text-primary-foreground">
      J
    </div>
  );
}

function CountPill({ count, className }: { count: number; className?: string }) {
  return (
    <span
      className={cn(
        "min-w-4 rounded-full bg-accent-tint px-1.5 text-center text-[10.5px] leading-4 font-semibold text-accent-strong",
        className
      )}
    >
      {count}
    </span>
  );
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const session = useSession();
  const [moreOpen, setMoreOpen] = useState(false);

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

  const badgeFor = (href: string) => (href === "/leads" && unassignedCount ? unassignedCount : null);
  const roleLabel = ROLE_LABELS[session.role as Role];

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 hidden h-screen w-17 shrink-0 flex-col border-r border-border bg-card md:flex lg:w-58">
        <div className="flex items-center gap-2.5 px-5 pt-4.5 pb-3.5 lg:px-4.5">
          <BrandMark />
          <span className="hidden text-[15px] font-bold tracking-tight lg:inline">Jualin CRM</span>
        </div>

        <div className="mb-1.5 hidden border-b border-border px-4.5 pb-3.5 text-xs text-muted-foreground lg:block">
          {session.organization_name}
        </div>

        <nav aria-label="Navigasi utama" className="flex flex-1 flex-col gap-px overflow-y-auto p-2.5">
          {NAV_ITEMS.map((item) => {
            const active = isActive(pathname, item.href);
            const Icon = NAV_ICONS[item.href];
            const badge = badgeFor(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                title={item.label}
                aria-label={badge ? `${item.label}, ${badge} tanpa pemilik aktif` : item.label}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative flex min-h-10 w-full items-center justify-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13.5px] transition-colors lg:justify-start",
                  active
                    ? "bg-accent-tint font-semibold text-accent-strong"
                    : "font-medium text-foreground/80 hover:bg-muted"
                )}
              >
                {Icon ? <Icon className="size-4.5 shrink-0 lg:size-4" aria-hidden /> : null}
                <span className="hidden flex-1 lg:inline">{item.label}</span>
                {badge ? (
                  <>
                    <CountPill count={badge} className="hidden lg:inline" />
                    {/* Icon-only rail: the count rides on the icon's corner. */}
                    <CountPill count={badge} className="absolute top-0.5 right-0.5 lg:hidden" />
                  </>
                ) : null}
              </Link>
            );
          })}
        </nav>

        <div className="border-t border-border p-3">
          <div className="flex flex-col items-center gap-2 px-0 py-1.5 lg:flex-row lg:gap-2.5 lg:px-2.5">
            <div
              title={`${session.full_name} · ${roleLabel}`}
              className="flex size-7 shrink-0 items-center justify-center rounded-full bg-accent-tint text-[11.5px] font-semibold text-accent-strong"
            >
              {initialsOf(session.full_name)}
            </div>
            <div className="hidden min-w-0 flex-1 lg:block">
              <div className="truncate text-[13px] font-medium">{session.full_name}</div>
              <div className="text-[11.5px] text-muted-foreground">{roleLabel}</div>
            </div>
            <button
              type="button"
              onClick={handleLogout}
              title="Keluar"
              aria-label="Keluar"
              className="flex cursor-pointer rounded-sm p-1.5 text-muted-foreground transition-colors hover:text-foreground"
            >
              <LogOut className="size-4" aria-hidden />
            </button>
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border bg-background/95 px-3.5 backdrop-blur-sm md:px-5 lg:px-7">
          <div className="flex min-w-0 items-center gap-2.5">
            <div className="md:hidden">
              <BrandMark />
            </div>
            <h1 className="truncate text-[15px] font-semibold">{pageTitle(pathname)}</h1>
          </div>
          <NotificationBell />
        </header>

        {/* One gutter, the same on every side at every width (#192): 14px
            phone, 20px tablet, 28px desktop — and the header's horizontal
            padding matches it, so the page title and the content share a
            left edge. Screens do NOT centre themselves in a max-width box:
            that made the side gutters 204px against a 28px top at 1920px.
            Reading-width screens (details, settings, docs) keep a max width
            but stay left-aligned.
            Bottom padding on phones clears the fixed bar plus the iPhone
            home indicator, so the last row of any page stays reachable. */}
        <main className="min-w-0 flex-1 bg-background p-3.5 pb-[calc(5.5rem+env(safe-area-inset-bottom))] md:p-5 lg:p-7">
          {children}
        </main>
      </div>

      <nav
        aria-label="Navigasi utama"
        className="fixed inset-x-0 bottom-0 z-40 grid grid-cols-5 border-t border-border bg-card pb-[env(safe-area-inset-bottom)] md:hidden"
      >
        {BOTTOM_NAV_ITEMS.map((item) => {
          const active = isActive(pathname, item.href);
          const Icon = NAV_ICONS[item.href];
          const badge = badgeFor(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={active ? "page" : undefined}
              aria-label={badge ? `${item.label}, ${badge} tanpa pemilik aktif` : undefined}
              className={cn(
                "relative flex min-h-14 flex-col items-center justify-center gap-1 text-[11px]",
                active ? "font-semibold text-accent-strong" : "font-medium text-muted-foreground"
              )}
            >
              {Icon ? <Icon className="size-5" aria-hidden /> : null}
              {item.label}
              {badge ? <CountPill count={badge} className="absolute top-1.5 left-1/2 ml-1.5" /> : null}
            </Link>
          );
        })}
        <button
          type="button"
          onClick={() => setMoreOpen(true)}
          aria-haspopup="dialog"
          className={cn(
            "flex min-h-14 cursor-pointer flex-col items-center justify-center gap-1 text-[11px]",
            isMoreActive(pathname) ? "font-semibold text-accent-strong" : "font-medium text-muted-foreground"
          )}
        >
          <Ellipsis className="size-5" aria-hidden />
          Lainnya
        </button>
      </nav>

      <Sheet open={moreOpen} onOpenChange={setMoreOpen}>
        <SheetContent>
          <SheetTitle className="sr-only">Lainnya</SheetTitle>
          <div className="flex items-center gap-3 border-b border-border pr-10 pb-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent-tint text-[13px] font-semibold text-accent-strong">
              {initialsOf(session.full_name)}
            </div>
            <div className="min-w-0">
              <div className="truncate text-[14px] font-semibold">{session.full_name}</div>
              <div className="truncate text-[12.5px] text-muted-foreground">
                {session.organization_name} · {roleLabel}
              </div>
            </div>
          </div>
          <nav aria-label="Menu lainnya" className="flex flex-col">
            {MORE_NAV_ITEMS.map((item) => {
              const active = isActive(pathname, item.href);
              const Icon = NAV_ICONS[item.href];
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  onClick={() => setMoreOpen(false)}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "flex min-h-12 items-center gap-3 rounded-md px-2 text-[14.5px]",
                    active ? "bg-accent-tint font-semibold text-accent-strong" : "font-medium text-foreground"
                  )}
                >
                  {Icon ? <Icon className="size-5 shrink-0" aria-hidden /> : null}
                  {item.label}
                </Link>
              );
            })}
          </nav>
          <button
            type="button"
            onClick={handleLogout}
            className="flex min-h-12 cursor-pointer items-center gap-3 rounded-md border-t border-border px-2 pt-1 text-[14.5px] font-medium text-destructive"
          >
            <LogOut className="size-5" aria-hidden />
            Keluar
          </button>
        </SheetContent>
      </Sheet>
    </div>
  );
}
