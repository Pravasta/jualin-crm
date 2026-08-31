"use client";

// Form management — the list screen (#89). Owner/Admin only: Manager and
// Employee get NO fetch at all (the gate sits above the useEffect that
// calls listForms), the same shape api-keys-screen.tsx uses for
// "mengetik URL langsung tidak menampilkan daftar" — nol panggilan API,
// not just a hidden button.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { listForms, type Form } from "@/lib/forms";
import { canManageForms } from "@/lib/form-permissions";
import { globalMessage } from "@/lib/auth-errors";
import { formatDateID } from "@/lib/date";
import { useSession } from "@/lib/session-context";
import { CreateFormDialog } from "./create-form-dialog";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function FormsScreen() {
  const session = useSession();
  const router = useRouter();
  const canManage = canManageForms(session.role);

  const [forms, setForms] = useState<Form[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  useEffect(() => {
    if (!canManage) return;
    const controller = new AbortController();
    listForms(controller.signal)
      .then((data) => {
        setForms(data);
        setError(null);
        setLoaded(true);
      })
      .catch((err) => {
        if (!isAbortError(err)) {
          setError(globalMessage(err));
          setLoaded(true);
        }
      });
    return () => controller.abort();
  }, [canManage]);

  if (!canManage) {
    return (
      <p className="text-[13px] text-muted-foreground">
        Pengelolaan formulir tidak tersedia untuk role Anda.
      </p>
    );
  }

  const loading = !loaded;

  return (
    <div>
      <button
        type="button"
        onClick={() => router.push("/connect")}
        className="mb-3.5 flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground"
      >
        ← Kembali ke Connect
      </button>

      <div className="mb-3.5 flex items-center justify-between">
        <div>
          <h2 className="text-[13.5px] font-semibold">Formulir</h2>
          <p className="text-[12.5px] text-muted-foreground">
            Salin satu potong HTML, tempel di situs Anda — lead masuk otomatis tanpa developer.
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>+ Buat formulir</Button>
      </div>

      <FormErrorBanner message={error} />

      {loading ? (
        <p className="text-[13px] text-muted-foreground">Memuat…</p>
      ) : forms.length === 0 ? (
        <p className="text-[13px] text-muted-foreground">
          Belum ada formulir. Buat satu untuk mulai menangkap lead dari situs Anda.
        </p>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <table className="w-full border-collapse">
            <thead>
              <tr className="bg-muted/40">
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Nama
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Submission
                </th>
                <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                  Dibuat
                </th>
                <th className="px-4 py-2.5" />
              </tr>
            </thead>
            <tbody>
              {forms.map((form) => (
                <tr
                  key={form.id}
                  className="cursor-pointer border-t border-border/70 hover:bg-muted/30"
                  onClick={() => router.push(`/connect/form/${form.id}`)}
                >
                  <td className="px-4 py-2.5">
                    <div className="text-[13px] font-medium">{form.name}</div>
                    <div className="font-mono text-[11.5px] text-muted-foreground">
                      {form.public_key}
                    </div>
                  </td>
                  <td className="px-4 py-2.5 text-[13px] text-foreground/70">{form.submit_count}</td>
                  <td className="px-4 py-2.5 text-[13px] text-foreground/70">
                    {formatDateID(form.created_at)}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <span className="text-[13px] text-accent-strong underline">Kelola</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <CreateFormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={(form) => {
          setCreateOpen(false);
          router.push(`/connect/form/${form.id}`);
        }}
      />
    </div>
  );
}
