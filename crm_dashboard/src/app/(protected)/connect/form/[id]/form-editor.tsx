"use client";

// Per-form editor (#89) — name, the six fixed fields (ADR-005: no form
// builder, only enabled/required/label per field), the domain allowlist,
// the copy-paste embed snippet, and deactivation. Owner/Admin only: the
// gate sits above the fetch, same as forms-screen.tsx and
// api-keys-screen.tsx.
//
// Forms have NO optimistic locking (formJSON carries no version, the
// backend PATCH is last-write-wins) — so unlike the lead/task edit forms
// (#33, #35) nothing here round-trips a version and there is no 409
// version_conflict path to handle.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FormErrorBanner } from "@/components/form-error-banner";
import { ApiError } from "@/lib/api-types";
import {
  FIELD_KEYS,
  FIELD_NAMES,
  firstFieldConfigError,
  getForm,
  updateForm,
  type FieldKey,
  type Fields,
  type Form,
} from "@/lib/forms";
import { canManageForms } from "@/lib/form-permissions";
import { autoResizeSnippet, fixedHeightSnippet, jsxSnippet } from "@/lib/form-snippet";
import { globalMessage } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";
import { DeactivateFormDialog } from "./deactivate-form-dialog";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

// A full origin, scheme + host, no path — the backend compares this
// verbatim against the browser's Origin header (form.originAllowed).
const ORIGIN_RE = /^https?:\/\/[^/\s]+$/;

export function FormEditor({ formId }: { formId: string }) {
  const router = useRouter();
  const session = useSession();
  const canManage = canManageForms(session.role);

  const [form, setForm] = useState<Form | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [name, setName] = useState("");
  const [fields, setFields] = useState<Fields | null>(null);
  const [origins, setOrigins] = useState<string[]>([]);
  const [newOrigin, setNewOrigin] = useState("");
  const [originHint, setOriginHint] = useState<string | null>(null);

  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<{ key: FieldKey; message: string } | null>(null);
  const [saved, setSaved] = useState(false);

  const [copied, setCopied] = useState<"auto" | "fixed" | "jsx" | null>(null);
  const [showJsx, setShowJsx] = useState(false);
  const [deactivateOpen, setDeactivateOpen] = useState(false);

  useEffect(() => {
    if (!canManage) return;
    const controller = new AbortController();
    getForm(formId, controller.signal)
      .then((data) => {
        setForm(data);
        setName(data.name);
        setFields(data.fields);
        setOrigins(data.allowed_origins);
        setNotFound(false);
        setLoadError(null);
      })
      .catch((err) => {
        if (isAbortError(err)) return;
        if (err instanceof ApiError && err.code === "not_found") setNotFound(true);
        else setLoadError(globalMessage(err));
      });
    return () => controller.abort();
  }, [formId, canManage]);

  if (!canManage) {
    return (
      <p className="text-[13px] text-muted-foreground">
        Pengelolaan formulir tidak tersedia untuk role Anda.
      </p>
    );
  }

  if (notFound) {
    return (
      <div className="flex flex-col items-center gap-3 py-16 text-center">
        <p className="text-sm text-muted-foreground">Formulir tidak ditemukan.</p>
        <Button variant="outline" onClick={() => router.push("/connect/form")}>
          Kembali ke daftar formulir
        </Button>
      </div>
    );
  }

  if (loadError) return <FormErrorBanner message={loadError} />;

  if (!form || !fields) {
    return <div className="py-16 text-center text-sm text-muted-foreground">Memuat…</div>;
  }

  const setFieldConfig = (key: FieldKey, patch: Partial<Fields[FieldKey]>) => {
    setSaved(false);
    setFields((prev) => {
      if (!prev) return prev;
      const next = { ...prev[key], ...patch };
      // Keep the two invariants the backend enforces coherent as the
      // user clicks, rather than only catching them at save: disabling a
      // field also clears "required"; marking one required also enables
      // it.
      if (patch.enabled === false) next.required = false;
      if (patch.required === true) next.enabled = true;
      return { ...prev, [key]: next };
    });
  };

  const addOrigin = () => {
    const value = newOrigin.trim().replace(/\/+$/, "");
    if (!value) return;
    if (!ORIGIN_RE.test(value)) {
      setOriginHint("Masukkan alamat lengkap dengan https://, tanpa path — contoh: https://tokosaya.com");
      return;
    }
    if (origins.includes(value)) {
      setOriginHint("Domain itu sudah ada di daftar.");
      return;
    }
    setOrigins((prev) => [...prev, value]);
    setNewOrigin("");
    setOriginHint(null);
    setSaved(false);
  };

  const removeOrigin = (value: string) => {
    setOrigins((prev) => prev.filter((o) => o !== value));
    setSaved(false);
  };

  async function handleSave() {
    setSaveError(null);
    setFieldError(null);
    if (name.trim() === "") {
      setSaveError("Nama formulir tidak boleh kosong.");
      return;
    }
    const configError = firstFieldConfigError(fields!);
    if (configError) {
      setFieldError(configError);
      return;
    }
    setSaving(true);
    try {
      const updated = await updateForm(formId, {
        name: name.trim(),
        fields: fields!,
        allowed_origins: origins,
      });
      setForm(updated);
      setName(updated.name);
      setFields(updated.fields);
      setOrigins(updated.allowed_origins);
      setSaved(true);
    } catch (err) {
      setSaveError(globalMessage(err));
    } finally {
      setSaving(false);
    }
  }

  async function copySnippet(kind: "auto" | "fixed" | "jsx", text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(kind);
      window.setTimeout(() => setCopied((c) => (c === kind ? null : c)), 2000);
    } catch {
      // Clipboard API can be unavailable (insecure context, older
      // browser) — the snippet stays visible and selectable below for a
      // manual copy.
    }
  }

  // The snippet reflects the PERSISTED form (form.name / form.public_key),
  // not the unsaved draft — copying a snippet with a name the Owner
  // hasn't saved yet would embed a title that doesn't match the form.
  const snippetParams = { publicKey: form.public_key, formName: form.name };
  const autoSnippet = autoResizeSnippet(snippetParams);
  const fixedSnippet = fixedHeightSnippet(snippetParams);

  return (
    <div className="max-w-2xl">
      <button
        type="button"
        onClick={() => router.push("/connect/form")}
        className="mb-3.5 flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground"
      >
        ← Kembali ke daftar formulir
      </button>

      <FormErrorBanner message={saveError} />

      <Card className="mb-3.5">
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="form-name">Nama formulir</Label>
            <Input
              id="form-name"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setSaved(false);
              }}
            />
          </div>

          <div>
            <div className="mb-1 text-[13px] font-medium">Field</div>
            <p className="mb-2.5 text-[12px] text-muted-foreground">
              Aktifkan field yang ingin ditampilkan dan ubah labelnya mengikuti bahasa bisnis Anda.
            </p>
            <div className="flex flex-col gap-2">
              {FIELD_KEYS.map((key) => {
                const cfg = fields[key];
                return (
                  <div key={key} className="rounded-md border border-border p-2.5">
                    <div className="flex items-center gap-3">
                      <label className="flex items-center gap-1.5 text-[13px]">
                        <input
                          type="checkbox"
                          checked={cfg.enabled}
                          onChange={(e) => setFieldConfig(key, { enabled: e.target.checked })}
                        />
                        {FIELD_NAMES[key]}
                      </label>
                      <label className="flex items-center gap-1.5 text-[12.5px] text-muted-foreground">
                        <input
                          type="checkbox"
                          checked={cfg.required}
                          disabled={!cfg.enabled}
                          onChange={(e) => setFieldConfig(key, { required: e.target.checked })}
                        />
                        Wajib diisi
                      </label>
                    </div>
                    {cfg.enabled && (
                      <div className="mt-2 flex flex-col gap-1">
                        <Input
                          aria-label={`Label untuk ${FIELD_NAMES[key]}`}
                          value={cfg.label}
                          placeholder="Label yang dilihat pengunjung"
                          onChange={(e) => setFieldConfig(key, { label: e.target.value })}
                        />
                        {fieldError?.key === key && (
                          <p className="text-[12px] text-destructive">{fieldError.message}</p>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          <div>
            <div className="mb-1 text-[13px] font-medium">Domain yang diizinkan</div>
            <p className="mb-2.5 text-[12px] text-muted-foreground">
              Formulir hanya bisa dipasang dan mengirim lead dari domain di daftar ini. Kosongkan dan
              formulir tidak bisa dipakai di mana pun.
            </p>
            {origins.length > 0 && (
              <ul className="mb-2 flex flex-col gap-1">
                {origins.map((origin) => (
                  <li
                    key={origin}
                    className="flex items-center justify-between rounded-md border border-border px-2.5 py-1.5 text-[13px]"
                  >
                    <span className="font-mono text-[12.5px]">{origin}</span>
                    <button
                      type="button"
                      onClick={() => removeOrigin(origin)}
                      className="text-[12px] text-muted-foreground hover:text-destructive"
                    >
                      Hapus
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <div className="flex gap-2">
              <Input
                value={newOrigin}
                placeholder="https://tokosaya.com"
                onChange={(e) => setNewOrigin(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addOrigin();
                  }
                }}
              />
              <Button type="button" variant="outline" onClick={addOrigin}>
                Tambah
              </Button>
            </div>
            {originHint && <p className="mt-1 text-[12px] text-destructive">{originHint}</p>}
          </div>

          <div className="flex items-center gap-3">
            <Button type="button" disabled={saving} onClick={handleSave}>
              {saving ? "Menyimpan…" : "Simpan perubahan"}
            </Button>
            {saved && <span className="text-[12.5px] text-accent-strong">Tersimpan</span>}
          </div>
        </CardContent>
      </Card>

      <Card className="mb-3.5">
        <CardContent className="flex flex-col gap-3">
          <div>
            <div className="mb-1 text-[13px] font-medium">Snippet embed</div>
            <p className="text-[12px] text-muted-foreground">
              Salin salah satu potongan di bawah dan tempel ke halaman situs Anda. Yang pertama
              menyesuaikan tingginya dengan isi formulir; yang kedua bertinggi tetap, untuk situs yang
              melarang script pihak ketiga.
            </p>
          </div>

          {origins.length === 0 && (
            <p className="rounded-md border border-destructive/35 bg-destructive/6 px-2.5 py-2 text-[12px] text-destructive">
              Tambahkan minimal satu domain di atas dan simpan — tanpa itu, formulir akan ditolak saat
              dipasang.
            </p>
          )}

          <SnippetBox
            label="Dianjurkan — tinggi menyesuaikan isi"
            code={autoSnippet}
            copied={copied === "auto"}
            onCopy={() => copySnippet("auto", autoSnippet)}
          />
          <SnippetBox
            label="Tanpa script — tinggi tetap"
            code={fixedSnippet}
            copied={copied === "fixed"}
            onCopy={() => copySnippet("fixed", fixedSnippet)}
          />

          <button
            type="button"
            onClick={() => setShowJsx((v) => !v)}
            className="self-start text-[12px] text-accent-strong underline"
          >
            {showJsx ? "Sembunyikan varian JSX" : "Memakai React / Next.js? Lihat varian JSX"}
          </button>
          {showJsx && (
            <SnippetBox
              label="JSX — untuk React / Next.js"
              code={jsxSnippet(snippetParams)}
              copied={copied === "jsx"}
              onCopy={() => copySnippet("jsx", jsxSnippet(snippetParams))}
            />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <div className="mb-1 text-[13px] font-medium">Nonaktifkan formulir</div>
          <p className="mb-3 text-[12px] text-muted-foreground">
            Formulir berhenti tampil di situs mana pun yang memasangnya. Tidak bisa dibatalkan.
          </p>
          <Button
            type="button"
            variant="outline"
            onClick={() => setDeactivateOpen(true)}
            className="border-destructive/35 bg-destructive/6 text-destructive hover:bg-destructive/10"
          >
            Nonaktifkan formulir
          </Button>
        </CardContent>
      </Card>

      <DeactivateFormDialog
        formId={form.id}
        formName={form.name}
        open={deactivateOpen}
        onOpenChange={setDeactivateOpen}
        onDeactivated={() => router.push("/connect/form")}
      />
    </div>
  );
}

function SnippetBox({
  label,
  code,
  copied,
  onCopy,
}: {
  label: string;
  code: string;
  copied: boolean;
  onCopy: () => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between">
        <span className="text-[12px] text-muted-foreground">{label}</span>
        <button
          type="button"
          onClick={onCopy}
          className="rounded-md border border-border px-2.5 py-1 text-[12px] text-foreground/70 hover:bg-muted"
        >
          {copied ? "Tersalin" : "Salin"}
        </button>
      </div>
      <pre className="overflow-x-auto rounded-md border border-border bg-muted/40 p-2.5 font-mono text-[11.5px] whitespace-pre-wrap">
        {code}
      </pre>
    </div>
  );
}
