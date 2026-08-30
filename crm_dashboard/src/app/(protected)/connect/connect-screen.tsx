// The landing page for the Connect surface (issue #86/ADR-012) — three
// capture-channel cards. Cards are shown to EVERY role unconditionally
// (keputusan D6, same as the nav item itself): the role gate lives
// inside the destination screen (canManageAPIKeys today, gated in
// api-keys-screen.tsx/api-docs-screen.tsx), not here. A Manager or
// Employee can open /connect/api and see "tidak tersedia untuk role
// Anda" — that's the intended shape, not a bug to fix by hiding the
// card.
import Link from "next/link";
import { KeyRound, FileText, Webhook } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";

interface ChannelCard {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
  href?: string;
  status: "active" | "unavailable";
}

// Webhook says "belum tersedia" — NOT "terkunci oleh paket". It simply
// doesn't exist yet (Phase 7); the locked-by-plan state is Phase 8
// (ADR-012 §4). Claiming it's locked now would be a lie about a
// capability the product doesn't have at all.
const CHANNELS: ChannelCard[] = [
  {
    icon: KeyRound,
    title: "API",
    description: "Kirim lead langsung dari sistem eksternal Anda lewat REST API dan API key.",
    href: "/connect/api",
    status: "active",
  },
  {
    icon: FileText,
    title: "Formulir",
    description: "Salin satu potong HTML, tempel di situs Anda — lead masuk otomatis tanpa developer.",
    href: "/connect/form",
    status: "active",
  },
  {
    icon: Webhook,
    title: "Webhook",
    description: "Terima event dari platform lain secara real-time.",
    status: "unavailable",
  },
];

export function ConnectScreen() {
  return (
    <div>
      <p className="mb-4.5 text-[13px] text-muted-foreground">
        Pilih cara pelanggan dan sistem eksternal mengirim lead ke organization Anda.
      </p>

      <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-3">
        {CHANNELS.map((channel) => {
          const Icon = channel.icon;
          const body = (
            <>
              <div className="mb-3 flex size-9 items-center justify-center rounded-md bg-accent-tint text-accent-strong">
                <Icon className="size-4.5" />
              </div>
              <div className="mb-1 flex items-center gap-2">
                <span className="text-[13.5px] font-semibold">{channel.title}</span>
                {channel.status === "unavailable" ? (
                  <span className="rounded-full bg-muted px-2 py-0.5 text-[10.5px] font-medium text-muted-foreground">
                    Belum tersedia
                  </span>
                ) : null}
              </div>
              <p className="text-[12.5px] text-muted-foreground">{channel.description}</p>
            </>
          );

          if (channel.status === "unavailable" || !channel.href) {
            return (
              <Card key={channel.title} className="opacity-60">
                <CardContent>{body}</CardContent>
              </Card>
            );
          }

          return (
            <Link key={channel.title} href={channel.href} className="block">
              <Card className="transition-colors hover:bg-muted/40">
                <CardContent>{body}</CardContent>
              </Card>
            </Link>
          );
        })}
      </div>
    </div>
  );
}
