import { Suspense } from "react";
import { ReportsScreen } from "./reports-screen";

// Suspense because the screen reads its filters from the URL
// (useSearchParams) — same shape as /leads.
export default function ReportsPage() {
  return (
    <Suspense fallback={null}>
      <ReportsScreen />
    </Suspense>
  );
}
