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
//
// Phase 8.6 (#166): three cards, never six. The handoff drew WhatsApp
// Business, Instagram DM, Marketplace and Google Sheets as channels with
// connect/disconnect toggles — dummy data, and a chat inbox is out of scope
// (scope.md). A locked card names no plan and offers no upgrade (brief §8.9).
import Link from "next/link";
import { ChevronRight, KeyRound, FileText, Lock, Webhook } from "lucide-react";
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
    <div className="mx-auto flex w-full max-w-[1280px] flex-col gap-3.5 md:gap-4">
      <p className="text-[14px] text-muted-foreground">
        Pilih cara pelanggan dan sistem eksternal mengirim lead ke organization Anda.
      </p>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {CHANNELS.map((channel) => {
          const Icon = channel.icon;
          const state =
            channel.productStatus === "unavailable"
              ? "unavailable"
              : channelCardState("active", session.plan, channel.channel);
          const open = channel.productStatus === "active" && state === "active";

          const body = (
            <>
              <div className="flex items-start justify-between gap-3">
                <div
                  className={
                    open
                      ? "flex size-10 items-center justify-center rounded-lg bg-accent-tint text-accent-strong"
                      : "flex size-10 items-center justify-center rounded-lg bg-muted text-muted-foreground"
                  }
                >
                  <Icon className="size-5" />
                </div>
                {state === "unavailable" && (
                  <span className="rounded-full bg-secondary px-2.5 py-0.5 text-[12px] font-bold text-secondary-foreground">
                    Belum tersedia
                  </span>
                )}
                {state === "locked" && (
                  <span className="inline-flex items-center gap-1 rounded-full bg-secondary px-2.5 py-0.5 text-[12px] font-bold text-secondary-foreground">
                    <Lock className="size-3" aria-hidden />
                    Terkunci oleh paket
                  </span>
                )}
              </div>
              <div className="mt-3 text-[16px] font-bold">{channel.title}</div>
              <p className="mt-1 text-[13.5px] text-muted-foreground">{channel.description}</p>
              {/* Visible and explains why — never hidden, never priced,
                  never a dead upgrade button (D6). Seeing this is the
                  point: a channel never seen is never upgraded for
                  (ADR-012, Alasan). */}
              {state === "locked" && (
                <p className="mt-3 border-t border-border pt-3 text-[13px] text-muted-foreground">
                  Kanal ini tidak termasuk paket Anda saat ini.
                </p>
              )}
              {open && (
                <div className="mt-3 flex items-center gap-1 border-t border-border pt-3 text-[13.5px] font-semibold text-accent-strong">
                  Kelola
                  <ChevronRight className="size-4" aria-hidden />
                </div>
              )}
            </>
          );

          if (!open) {
            return (
              <div
                key={channel.title}
                aria-disabled="true"
                className="flex flex-col rounded-xl border border-border bg-muted/40 p-4 opacity-70"
              >
                {body}
              </div>
            );
          }

          return (
            <Link
              key={channel.title}
              href={channel.href}
              className="flex flex-col rounded-xl border border-border bg-card p-4 transition-colors hover:border-primary"
            >
              {body}
            </Link>
          );
        })}
      </div>
    </div>
  );
}
