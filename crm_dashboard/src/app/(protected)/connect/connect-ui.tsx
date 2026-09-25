"use client";

// The frame shared by the three Connect channel screens and their detail
// pages (#166). Four screens drew the same back link, header, "not for your
// role" box, loading line and empty line by hand; they drifted in small ways
// (one said "Memuat…" as a paragraph, another as a centered box). This file
// is local to connect/ on purpose — other sections have their own shapes and
// no second real caller yet (Aturan #28).
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

export function BackLink({ href, label }: { href: string; label: string }) {
  return (
    <Link
      href={href}
      className="flex min-h-9 items-center gap-1.5 self-start text-[13.5px] font-medium text-muted-foreground hover:text-foreground"
    >
      <ArrowLeft className="size-4" aria-hidden />
      {label}
    </Link>
  );
}

// Title + one-line explanation, with the section's actions beside it from
// 640px and stacked under it on a phone, so no button is squeezed off-screen.
export function SectionHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0">
        <h2 className="text-[16px] font-bold">{title}</h2>
        {description && <p className="mt-0.5 text-[13.5px] text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 flex-wrap gap-2">{actions}</div>}
    </div>
  );
}

// Brief §4.1: a screen a role cannot use says so — not an empty page, not an
// error.
export function NotForRole({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto w-full max-w-[1280px] rounded-xl border border-dashed border-border bg-card px-6 py-12 text-center text-[14px] text-muted-foreground">
      {children}
    </div>
  );
}

export function ListSkeleton({ label, rows = 3 }: { label: string; rows?: number }) {
  return (
    <div aria-busy="true" aria-label={label} className="flex flex-col gap-2">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="h-16 animate-pulse rounded-[10px] bg-muted md:h-13" />
      ))}
    </div>
  );
}

export function EmptyCard({ title, children }: { title: string; children?: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-border bg-card px-5 py-12 text-center">
      <div className="mb-1.5 text-[16px] font-bold">{title}</div>
      {children && <p className="mx-auto max-w-[42ch] text-[14px] text-muted-foreground">{children}</p>}
    </div>
  );
}

export const tableHeadRow =
  "bg-muted text-left text-[11.5px] font-bold tracking-[0.05em] text-muted-foreground uppercase";
