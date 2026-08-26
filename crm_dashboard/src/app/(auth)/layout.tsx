// Auth screens never call GET /v1/me — there's no guaranteed session yet.
// Deliberately a separate route group from (protected), which always
// checks it (see (protected)/layout.tsx).
export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-1 items-center justify-center bg-muted/30 p-4">
      <div className="w-full max-w-sm">{children}</div>
    </div>
  );
}
