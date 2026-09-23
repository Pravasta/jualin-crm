import type { CSSProperties } from "react";

import { STATUS_META, type LeadStatus, type StatusShape } from "@/lib/labels";
import { cn } from "@/lib/utils";

// Lead status badge — token sheet "Opsi B" (issue #159): colored text and
// edge on white, with the SHAPE carrying meaning so eight statuses stay
// distinguishable without relying on color (design brief §5.2, §7.1).
// Icons are the mobile treatment (Opsi A); in a 25-row table eight icons
// turn into noise, so the dashboard uses shape instead.

const SHAPE_CLASS: Record<StatusShape, string> = {
  pill: "rounded-full border-[1.5px] border-solid font-semibold",
  square: "rounded-[5px] border-2 border-solid font-bold",
  // Tidak Memenuhi Syarat and Spam share this shape — both are excluded
  // from conversion rate — and are told apart by color and label.
  dashed: "rounded-[5px] border-[1.5px] border-dashed font-semibold",
};

export function StatusBadge({
  status,
  size = "sm",
  className,
}: {
  status: LeadStatus;
  size?: "sm" | "md";
  className?: string;
}) {
  const meta = STATUS_META[status];
  const style: CSSProperties = { color: meta.color, borderColor: meta.color };
  return (
    <span
      className={cn(
        "inline-flex items-center whitespace-nowrap bg-white",
        size === "sm" ? "px-2.5 py-0.5 text-[12.5px]" : "px-3 py-1 text-[13px]",
        SHAPE_CLASS[meta.shape],
        className
      )}
      style={style}
    >
      {meta.label}
    </span>
  );
}
