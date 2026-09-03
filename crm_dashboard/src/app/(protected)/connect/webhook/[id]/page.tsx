import { WebhookEditor } from "./webhook-editor";

// Server Component only to unwrap the async `params` (Next.js 15+), same
// as connect/form/[id]/page.tsx — the session comes from GET /v1/me via
// SessionGate, not SSR.
export default async function WebhookEditorPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <WebhookEditor endpointId={id} />;
}
