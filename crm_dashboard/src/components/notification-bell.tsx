"use client";

// Lives in the topbar (app-shell.tsx), not the Team screen — notifications
// are global to every page, unlike the rest of issue #34. Design brief
// §7.4 groups it with Team in the inventory, but visually it's part of
// the shell built in #40.
//
// "Tidak termasuk" (issue #34): no realtime/websocket. Fetched once on
// mount and again whenever the panel opens — freeze forbids a message
// broker, and polling on an interval nobody asked for would be adding
// infrastructure for a problem that hasn't come up yet (Aturan #27).
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Bell } from "lucide-react";
import {
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  type Notification,
} from "@/lib/notifications";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

function formatRelativeTime(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const minutes = Math.floor(diffMs / 60000);
  if (minutes < 1) return "Baru saja";
  if (minutes < 60) return `${minutes} menit lalu`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} jam lalu`;
  const days = Math.floor(hours / 24);
  if (days === 1) return "Kemarin";
  return `${days} hari lalu`;
}

export function NotificationBell() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    listNotifications({}, controller.signal)
      .then(setNotifications)
      .catch((err) => {
        if (!isAbortError(err)) setNotifications([]);
      });
    return () => controller.abort();
  }, [refreshKey]);

  const unreadCount = notifications.filter((n) => !n.read_at).length;

  async function handleMarkAllRead() {
    await markAllNotificationsRead().catch(() => {});
    setRefreshKey((k) => k + 1);
  }

  async function handleClickNotification(n: Notification) {
    if (!n.read_at) {
      await markNotificationRead(n.id).catch(() => {});
      setRefreshKey((k) => k + 1);
    }
    setOpen(false);
    if (n.lead_id) router.push(`/leads/${n.lead_id}`);
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-label="Notifikasi"
        className="relative flex cursor-pointer rounded-md p-1.5 text-foreground/70 hover:bg-muted"
      >
        <Bell className="size-4.5" aria-hidden />
        {unreadCount > 0 && (
          <span className="absolute top-1 right-1 size-2 rounded-full border border-background bg-primary" />
        )}
      </button>

      {open && (
        <div className="absolute top-10 right-0 z-50 w-85 overflow-hidden rounded-lg border border-border bg-background shadow-lg">
          <div className="flex items-center justify-between border-b border-border px-3.5 py-3">
            <span className="text-[13.5px] font-semibold">Notifikasi</span>
            {unreadCount > 0 && (
              <button
                type="button"
                onClick={handleMarkAllRead}
                className="text-xs font-medium text-accent-strong"
              >
                Tandai semua dibaca
              </button>
            )}
          </div>
          <div className="max-h-90 overflow-y-auto">
            {notifications.length === 0 && (
              <p className="px-3.5 py-6 text-center text-[12.5px] text-muted-foreground">
                Belum ada notifikasi.
              </p>
            )}
            {notifications.map((n) => (
              <button
                key={n.id}
                type="button"
                onClick={() => handleClickNotification(n)}
                className="flex w-full gap-2.5 border-b border-border/60 px-3.5 py-2.5 text-left last:border-b-0 hover:bg-muted/50"
                style={{ background: n.read_at ? undefined : "oklch(0.56 0.19 41 / 4%)" }}
              >
                <span
                  className="mt-1.5 size-1.5 shrink-0 rounded-full bg-primary"
                  style={{ visibility: n.read_at ? "hidden" : "visible" }}
                />
                <span>
                  <div className="text-[13px]">{n.title}</div>
                  <div className="mt-0.5 text-[11.5px] text-muted-foreground">
                    {formatRelativeTime(n.created_at)}
                  </div>
                </span>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
