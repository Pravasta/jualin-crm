// Shared between the list and the editor so an event is named the same
// way in both places — the raw "lead.status_changed" is for the wire, not
// for someone deciding what an endpoint does.
import { WEBHOOK_EVENT_LABELS, type WebhookEvent } from "@/lib/webhooks";

export function WebhookEventBadges({ events }: { events: WebhookEvent[] }) {
  if (events.length === 0) {
    return <span className="text-[12.5px] text-muted-foreground">—</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {events.map((event) => (
        <span
          key={event}
          className="rounded border border-border bg-muted/40 px-1.5 py-0.5 text-[11.5px] text-foreground/80"
        >
          {WEBHOOK_EVENT_LABELS[event] ?? event}
        </span>
      ))}
    </div>
  );
}
