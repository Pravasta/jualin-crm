"use client";

// The landing page for the Connect surface (issue #86/ADR-012) — three
// capture-channel cards. Cards are shown to EVERY role unconditionally
// (keputusan D6, same as the nav item itself): the role gate lives
// inside the destination screen (canManageAPIKeys today, gated in
// api-keys-screen.tsx/api-docs-screen.tsx), not here. A Manager or
// Employee can open /connect/api and see "tidak tersedia untuk role
// Anda" — that's the intended shape, not a bug to fix by hiding the
// card.
//
// Client component since #114 — rendering the locked-by-plan state
// needs session.plan, which only exists inside SessionGate.
import Link from "next/link";
import { KeyRound, FileText, Webhook } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { useSession } from "@/lib/session-context";
import { channelCardState, type PlanChannel } from "@/lib/plan";

type ChannelCard =
  | {
      icon: React.ComponentType<{ className?: string }>;
      title: string;
      description: string;
      href: string;
      productStatus: "active";
      channel: PlanChannel;
    }
  | {
      icon: React.ComponentType<{ className?: string }>;
      title: string;
      description: string;
      productStatus: "unavailable";
    };

// All three channels are live as of Phase 7 (#103). "unavailable" is
// kept as a card state (TD §8) for the next channel to ship (inbound
// webhook, Phase 7.5) — no entry uses it today, and it must never be
// reused for locked-by-plan: that would tell an organization a channel
// doesn't exist when it actually does, just not on their plan.
//
// Webhook is the only OUTBOUND one: the other two bring leads in, this
// one sends events out. Its description said "Terima event dari platform
// lain" while it was still a placeholder, which described inbound webhook
// — Phase 7.5, a different feature entirely. Corrected here rather than
// left to read as a promise the screen does not keep.
const CHANNELS: ChannelCard[] = [
  {
    icon: KeyRound,
    title: "API",
    description: "Kirim lead langsung dari sistem eksternal Anda lewat REST API dan API key.",
    href: "/connect/api",
    productStatus: "active",
    channel: "api_key",
  },
  {
    icon: FileText,
    title: "Formulir",
    description: "Salin satu potong HTML, tempel di situs Anda — lead masuk otomatis tanpa developer.",
    href: "/connect/form",
    productStatus: "active",
    channel: "form",
  },
  {
    icon: Webhook,
    title: "Webhook",
    description:
      "Kirim event ke sistem Anda sendiri begitu lead masuk atau statusnya berubah — tanpa perlu menanyakannya berulang kali.",
    href: "/connect/webhook",
    productStatus: "active",
    channel: "webhook",
  },
];

export function ConnectScreen() {
  const session = useSession();

  return (
    <div>
      <p className="mb-4.5 text-[13px] text-muted-foreground">
        Pilih cara pelanggan dan sistem eksternal mengirim lead ke organization Anda.
      </p>

      <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-3">
        {CHANNELS.map((channel) => {
          const Icon = channel.icon;
          const state =
            channel.productStatus === "unavailable"
              ? "unavailable"
              : channelCardState("active", session.plan, channel.channel);

          const body = (
            <>
              <div className="mb-3 flex size-9 items-center justify-center rounded-md bg-accent-tint text-accent-strong">
                <Icon className="size-4.5" />
              </div>
              <div className="mb-1 flex items-center gap-2">
                <span className="text-[13.5px] font-semibold">{channel.title}</span>
                {state === "unavailable" ? (
                  <span className="rounded-full bg-muted px-2 py-0.5 text-[10.5px] font-medium text-muted-foreground">
                    Belum tersedia
                  </span>
                ) : null}
                {state === "locked" ? (
                  <span className="rounded-full bg-muted px-2 py-0.5 text-[10.5px] font-medium text-muted-foreground">
                    Terkunci oleh paket
                  </span>
                ) : null}
              </div>
              <p className="text-[12.5px] text-muted-foreground">{channel.description}</p>
              {/* Visible and explains why — never hidden, never priced,
                  never a dead upgrade button (D6). Seeing this is the
                  point: a channel never seen is never upgraded for
                  (ADR-012, Alasan). */}
              {state === "locked" ? (
                <p className="mt-2 text-[12.5px] text-muted-foreground">
                  Kanal ini tidak termasuk paket Anda saat ini.
                </p>
              ) : null}
            </>
          );

          if (channel.productStatus !== "active" || state !== "active") {
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
