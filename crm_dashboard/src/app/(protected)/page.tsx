"use client";

import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { logout } from "@/lib/auth";
import { useSession } from "@/lib/session-context";

// Deliberately minimal — issue #31's boundary is "bisa login, belum ada
// layar lead" (docs/phases/03-owner-dashboard/issues.md). Lead list
// lands in #32. A working logout control is still required here: it's
// the other half of #31's own acceptance criterion ("mendaftar →
// verifikasi → masuk → KELUAR, seluruhnya lewat layar").
export default function DashboardHomePage() {
  const router = useRouter();
  const session = useSession();

  async function handleLogout() {
    await logout().catch(() => {
      // logout always succeeds from the client's perspective — crm_be's
      // handler answers 204 unconditionally (see internal/auth's
      // not-found-is-success reasoning).
    });
    router.push("/login");
  }

  return (
    <div className="flex flex-1 flex-col gap-4 p-8">
      <h1 className="text-lg font-medium">
        Selamat datang, {session.full_name}
      </h1>
      <p className="text-sm text-muted-foreground">{session.organization_name}</p>
      <Button variant="outline" className="w-fit" onClick={handleLogout}>
        Keluar
      </Button>
    </div>
  );
}
