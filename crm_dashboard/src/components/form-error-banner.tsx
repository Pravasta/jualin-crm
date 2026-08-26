import { Alert, AlertDescription } from "@/components/ui/alert";

// error.message shown apa adanya — backend already writes it in
// Indonesian (TD phase 3 §5), never re-translated here.
export function FormErrorBanner({ message }: { message?: string | null }) {
  if (!message) return null;
  return (
    <Alert variant="destructive">
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  );
}
