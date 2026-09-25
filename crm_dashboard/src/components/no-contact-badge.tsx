// "Belum ada kontak" — the marker freeze.md promised for a lead nobody can
// call or email (issue #143). Text, not colour alone, and neutral rather than
// red: a lead without contact is a legitimate state (a form that never asked,
// an integration that never sent one), not an error the user caused.
export function NoContactBadge({ className = "" }: { className?: string }) {
  return (
    <span
      className={`inline-block rounded-[4px] bg-secondary px-1.5 py-px text-[11px] font-semibold whitespace-nowrap text-secondary-foreground ${className}`}
    >
      Belum ada kontak
    </span>
  );
}
