import { LeadDetail } from "./lead-detail";

// A Server Component only to unwrap the async `params` (Next.js 15+) —
// everything below it is client-rendered, same as every other screen in
// this app (session comes from GET /v1/me via SessionGate, not SSR).
export default async function LeadDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <LeadDetail leadId={id} />;
}
