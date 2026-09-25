// Auth screens never call GET /v1/me — there's no guaranteed session yet.
// Deliberately a separate route group from (protected), which always
// checks it (see (protected)/layout.tsx).
//
// Phase 8.6 (#168): the brand sits above a narrow card; on a phone the card
// starts near the top rather than dead center, so the on-screen keyboard
// doesn't push the submit button out of view. The six screens share their
// title size and link color from here instead of each restating it — they
// all use CardTitle and plain <Link>s.
export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-1 flex-col items-center bg-background px-4 pt-10 pb-8 md:justify-center md:pt-8">
      <div className="mb-6 flex items-center gap-2.5">
        <div className="flex size-9 items-center justify-center rounded-lg bg-primary text-[17px] font-bold text-primary-foreground">
          J
        </div>
        <span className="text-[19px] font-extrabold tracking-tight">Jualin CRM</span>
      </div>
      <div className="w-full max-w-sm [&_[data-slot=card-title]]:text-[20px] [&_[data-slot=card-title]]:font-bold [&_a]:font-semibold [&_a]:text-accent-strong">
        {children}
      </div>
    </div>
  );
}
