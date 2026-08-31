import { FormEditor } from "./form-editor";

// Server Component only to unwrap the async `params` (Next.js 15+), same
// as leads/[id]/page.tsx and customers/[id]/page.tsx — the session comes
// from GET /v1/me via SessionGate, not SSR.
export default async function FormEditorPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <FormEditor formId={id} />;
}
