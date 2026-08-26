import { CustomerDetail } from "./customer-detail";

// A Server Component only to unwrap the async `params` (Next.js 15+),
// same as leads/[id]/page.tsx — session comes from GET /v1/me via
// SessionGate, not SSR.
export default async function CustomerDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <CustomerDetail customerId={id} />;
}
