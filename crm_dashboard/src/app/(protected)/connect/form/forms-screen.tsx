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
import { BackLink, EmptyCard, ListSkeleton, NotForRole, SectionHeader, tableHeadRow } from "../connect-ui";
import Link from "next/link";
import { Plus } from "lucide-react";

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
    return <NotForRole>Pengelolaan formulir tidak tersedia untuk role Anda.</NotForRole>;
  }

  const loading = !loaded;

  return (
    <div className="flex w-full flex-col gap-3.5 md:gap-4">
      <BackLink href="/connect" label="Connect" />
      <SectionHeader
        title="Formulir"
        description="Salin satu potong HTML, tempel di situs Anda — lead masuk otomatis tanpa developer."
        actions={
          <Button onClick={() => setCreateOpen(true)} className="gap-1.5 md:h-9 md:px-4">
            <Plus className="size-4" aria-hidden />
            Buat formulir
          </Button>
        }
      />

      <FormErrorBanner message={error} />

      {loading ? (
        <ListSkeleton label="Memuat formulir" />
      ) : forms.length === 0 ? (
        <EmptyCard title="Belum ada formulir">
          Buat satu untuk mulai menangkap lead dari situs Anda.
        </EmptyCard>
      ) : (
        <>
          <div className="hidden overflow-hidden rounded-xl border border-border bg-card md:block">
            <table className="w-full table-fixed border-collapse text-[14px]">
              <thead>
                <tr className={tableHeadRow}>
                  <th className="px-4 py-2.5">Nama</th>
                  <th className="w-32 px-3 py-2.5">Submission</th>
                  <th className="w-36 px-3 py-2.5">Dibuat</th>
                  <th className="w-24 px-4 py-2.5" aria-label="Aksi" />
                </tr>
              </thead>
              <tbody>
                {forms.map((form) => (
                  <tr
                    key={form.id}
                    className="cursor-pointer border-t border-border/60 hover:bg-muted/50"
                    onClick={() => router.push(`/connect/form/${form.id}`)}
                  >
                    <td className="px-4 py-3">
                      <div className="truncate font-semibold">{form.name}</div>
                      <div className="truncate font-mono text-[12.5px] text-muted-foreground">{form.public_key}</div>
                    </td>
                    <td className="px-3 py-3 tabular-nums">{form.submit_count}</td>
                    <td className="px-3 py-3 whitespace-nowrap text-muted-foreground">{formatDateID(form.created_at)}</td>
                    <td className="px-4 py-3 text-right">
                      <Link
                        href={`/connect/form/${form.id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="font-semibold text-accent-strong hover:underline"
                      >
                        Kelola
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <ul className="flex flex-col gap-2.5 md:hidden">
            {forms.map((form) => (
              <li key={form.id}>
                <Link
                  href={`/connect/form/${form.id}`}
                  className="block rounded-[10px] border border-border bg-card p-3.5 active:bg-muted/60"
                >
                  <div className="truncate text-[15px] font-bold">{form.name}</div>
                  <div className="truncate font-mono text-[12.5px] text-muted-foreground">{form.public_key}</div>
                  <div className="mt-2 flex flex-wrap gap-x-3 text-[13px] text-muted-foreground">
                    <span>{form.submit_count} submission</span>
                    <span>Dibuat {formatDateID(form.created_at)}</span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        </>
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
