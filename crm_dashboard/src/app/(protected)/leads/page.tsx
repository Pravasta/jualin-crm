import { Suspense } from "react";
import { LeadsList } from "./leads-list";

export default function LeadsPage() {
  return (
    <Suspense fallback={null}>
      <LeadsList />
    </Suspense>
  );
}
