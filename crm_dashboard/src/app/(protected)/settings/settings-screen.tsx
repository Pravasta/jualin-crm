"use client";

// The design's SETTINGS section shows both org name and full name as
// editable inputs with working-looking "Simpan" buttons — but
// `renderSettingsVals()` in the mockup's own source is a literal empty
// object, every input uses an uncontrolled defaultValue with no
// onChange, and neither button has an onClick. It was never wired up
// even in the prototype. More importantly, crm_be has no PATCH endpoint
// for organization or user profile at all (TD §8's screen map lists
// only "GET /v1/me" for this screen, and the issue checklist doesn't
// mention a write either) — so this is a read-only display of the
// session already available via useSession(), keeping the two-card
// visual grouping but dropping the fake edit affordance entirely.
//
// The "Integrasi API" card that used to link into API key management
// (issue #48) is REMOVED here, not just re-pointed — issue #86/ADR-012
// moves that capability out from under Pengaturan entirely, into its
// own top-level `Connect` menu item. Leaving a second entry point here
// would contradict the ADR's own reasoning ("Kenapa bukan tetap di
// dalam Pengaturan"): Settings is for account/org configuration rarely
// touched after setup, Connect is where the product's capture layer
// lives. See src/app/(protected)/connect/connect-screen.tsx.
import { Card, CardContent } from "@/components/ui/card";
import { ROLE_LABELS, type Role } from "@/lib/labels";
import { useSession } from "@/lib/session-context";

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-[13.5px]">{value}</span>
    </div>
  );
}

export function SettingsScreen() {
  const session = useSession();

  return (
    <div className="flex max-w-md flex-col gap-4">
      <Card>
        <CardContent className="flex flex-col gap-3">
          <div className="text-[13.5px] font-semibold">Organization</div>
          <Field label="Nama organization" value={session.organization_name} />
        </CardContent>
      </Card>

      <Card>
        <CardContent className="flex flex-col gap-3">
          <div className="text-[13.5px] font-semibold">Profil Anda</div>
          <Field label="Nama lengkap" value={session.full_name} />
          <Field label="Email" value={session.email} />
          <Field label="Role" value={ROLE_LABELS[session.role as Role]} />
        </CardContent>
      </Card>
    </div>
  );
}
