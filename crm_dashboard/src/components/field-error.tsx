// Renders directly under the field it belongs to — validation_failed's
// details[] must land here, never as a global toast (TD phase 3 §5).
export function FieldError({ message }: { message?: string }) {
  if (!message) return null;
  return <p className="text-sm text-destructive">{message}</p>;
}
