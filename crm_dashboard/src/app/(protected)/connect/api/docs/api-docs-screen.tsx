"use client";

// The customer-facing integration docs page (issue #49, TD §13) — freeze
// is explicit this isn't a nice-to-have: "kualitas halaman ini berdampak
// langsung ke biaya support" in a low-price product. Lives inside the
// dashboard, next to the key screen (TD §13: "bukan berkas Markdown di
// repo") — the reader is a customer's developer who just opened the
// dashboard to copy a key.
//
// This page can NEVER show a genuinely working curl example — it has no
// secret to put in one, only key_prefix (Aturan #21: the raw secret
// exists nowhere after the create dialog closes). The one place a
// working example truly exists is create-api-key-dialog.tsx's reveal
// step, which has the real secret in hand for that one moment. This
// page instead shows the same format with a placeholder and points
// unmistakably at that flow, which is what makes acceptance criterion
// #10 ("mengikuti halaman dari nol") honest rather than aspirational.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { FormErrorBanner } from "@/components/form-error-banner";
import { buildCurlExample } from "@/lib/api-docs";
import { canManageAPIKeys } from "@/lib/api-key-rows";
import { listAPIKeys, type APIKey } from "@/lib/api-keys";
import { globalMessage } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";
import { CreateAPIKeyDialog } from "../create-api-key-dialog";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-2.5">
        <h2 className="text-[13.5px] font-semibold">{title}</h2>
        <div className="flex flex-col gap-2 text-[13px] text-foreground/90">{children}</div>
      </CardContent>
    </Card>
  );
}

export function APIDocsScreen() {
  const session = useSession();
  const router = useRouter();
  const canManage = canManageAPIKeys(session.role);

  // Active keys only — a revoked key's prefix would make the example
  // look correct while being guaranteed to fail (401) if run.
  const [activeKeys, setActiveKeys] = useState<APIKey[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loadedKey, setLoadedKey] = useState(-1);
  const [refreshKey, setRefreshKey] = useState(0);
  const [selectedId, setSelectedId] = useState<string>("");
  const [createOpen, setCreateOpen] = useState(false);

  useEffect(() => {
    if (!canManage) return;
    const controller = new AbortController();
    listAPIKeys(controller.signal)
      .then((keys) => {
        const active = keys.filter((k) => k.revoked_at === null);
        setActiveKeys(active);
        setSelectedId((current) => current || active[0]?.id || "");
        setError(null);
        setLoadedKey(refreshKey);
      })
      .catch((err) => {
        if (!isAbortError(err)) setError(globalMessage(err));
      });
    return () => controller.abort();
  }, [refreshKey, canManage]);

  if (!canManage) {
    return (
      <p className="text-[13px] text-muted-foreground">
        Halaman dokumentasi integrasi tidak tersedia untuk role Anda.
      </p>
    );
  }

  const loading = loadedKey !== refreshKey;
  const selected = activeKeys.find((k) => k.id === selectedId) ?? null;
  const exampleCredential = selected ? `${selected.key_prefix}...<secret_anda>` : "<key_prefix_anda>...<secret_anda>";

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-[15px] font-semibold">Dokumentasi integrasi</h1>
        <Button variant="outline" onClick={() => router.push("/connect/api")}>
          ← Kembali ke API Key
        </Button>
      </div>

      <FormErrorBanner message={error} />

      <Section title="Kunci Anda">
        {loading ? (
          <p className="text-muted-foreground">Memuat…</p>
        ) : activeKeys.length === 0 ? (
          <div className="flex flex-col gap-2">
            <p className="text-muted-foreground">
              Anda belum punya kunci aktif. Buat satu dulu — begitu kunci dibuat, dialognya akan
              menampilkan perintah <code className="font-mono">curl</code> yang sudah terisi lengkap dan
              langsung bisa dijalankan.
            </p>
            <Button className="w-fit" onClick={() => setCreateOpen(true)}>
              + Buat kunci baru
            </Button>
          </div>
        ) : (
          <div className="flex flex-col gap-1.5">
            <label className="text-[12.5px] text-muted-foreground" htmlFor="docs-key-select">
              Contoh di bawah ini memakai kunci:
            </label>
            <select
              id="docs-key-select"
              value={selectedId}
              onChange={(e) => setSelectedId(e.target.value)}
              className="h-8.5 w-fit rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              {activeKeys.map((k) => (
                <option key={k.id} value={k.id}>
                  {k.name} ({k.key_prefix}…)
                </option>
              ))}
            </select>
          </div>
        )}
      </Section>

      <Section title="Contoh perintah">
        <pre className="overflow-x-auto rounded-md border border-border bg-muted/40 p-2.5 font-mono text-[11.5px] whitespace-pre-wrap">
          {buildCurlExample(API_BASE_URL, exampleCredential)}
        </pre>
        <p className="text-muted-foreground">
          Ganti <code className="font-mono">&lt;secret_anda&gt;</code> dengan kunci lengkap yang Anda
          salin saat membuatnya. Kami tidak menyimpan raw kunci Anda — begitu dialog pembuatan ditutup,
          kami sendiri pun tidak bisa menampilkannya lagi.
        </p>
      </Section>

      <Section title="Autentikasi">
        <p>
          Kirim kunci Anda lewat header <code className="font-mono">Authorization: Bearer</code>. Setiap
          kunci berbentuk <code className="font-mono">jln_live_&lt;key_id&gt;_&lt;secret&gt;</code> —
          seluruhnya harus dikirim persis seperti saat ditampilkan, tanpa dipotong atau diubah.
        </p>
        <div className="rounded-md border border-amber-500/30 bg-amber-500/10 p-2.5 text-[12.5px]">
          <span className="font-semibold">Jangan pernah panggil endpoint ini dari JavaScript di
          browser.</span>{" "}
          Domain situs Anda tidak pernah masuk daftar asal yang kami izinkan untuk berbagi kredensial
          (CORS) — panggilan dari browser akan gagal sebelum responsnya terbaca. Yang lebih penting:
          menempelkan kunci ini di kode sisi klien berarti kunci itu bisa dibaca siapa pun yang membuka
          situs Anda. Kirim selalu dari server Anda sendiri.
        </div>
      </Section>

      <Section title="Field yang diterima">
        <table className="w-full border-collapse text-[12.5px]">
          <thead>
            <tr className="border-b border-border text-left text-muted-foreground">
              <th className="py-1.5 pr-3 font-medium">Field</th>
              <th className="py-1.5 font-medium">Keterangan</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">name</td>
              <td className="py-1.5">Wajib.</td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">email, phone, company, notes</td>
              <td className="py-1.5">Opsional.</td>
            </tr>
            <tr>
              <td className="py-1.5 pr-3 font-mono">(field lain apa pun)</td>
              <td className="py-1.5">
                Tetap tersimpan apa adanya untuk keperluan Anda sendiri — tidak divalidasi, tidak
                ditolak, dan tidak akan membuat request gagal.
              </td>
            </tr>
          </tbody>
        </table>
      </Section>

      <Section title="Katalog kesalahan">
        <table className="w-full border-collapse text-[12.5px]">
          <thead>
            <tr className="border-b border-border text-left text-muted-foreground">
              <th className="py-1.5 pr-3 font-medium">HTTP</th>
              <th className="py-1.5 pr-3 font-medium font-mono">code</th>
              <th className="py-1.5 font-medium">Arti &amp; tindakan</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3">400</td>
              <td className="py-1.5 pr-3 font-mono">validation_failed</td>
              <td className="py-1.5">
                <code className="font-mono">name</code> kosong atau bentuk body salah. Periksa{" "}
                <code className="font-mono">details</code> di response untuk field yang persis salah.
              </td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3">401</td>
              <td className="py-1.5 pr-3 font-mono">invalid_api_key</td>
              <td className="py-1.5">
                Kunci tidak dikenal, sudah dicabut, atau salah ketik. Buat kunci baru dari layar API Key
                bila ini terjadi terus — kunci yang dicabut tidak bisa diaktifkan lagi.
              </td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3">403</td>
              <td className="py-1.5 pr-3 font-mono">insufficient_scope</td>
              <td className="py-1.5">
                Kunci Anda hanya bisa membuat lead — tidak bisa mengelola tim, membuat kunci lain, atau
                memilih siapa yang menerima lead ini (jangan kirim field penugasan).
              </td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3">413</td>
              <td className="py-1.5 pr-3 font-mono">payload_too_large</td>
              <td className="py-1.5">Body melebihi 64 KB. Kirim data lead, bukan lampiran besar.</td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3">429</td>
              <td className="py-1.5 pr-3 font-mono">rate_limited</td>
              <td className="py-1.5">
                Lihat bagian &ldquo;Batas kecepatan&rdquo; di bawah.
              </td>
            </tr>
            <tr>
              <td className="py-1.5 pr-3">500</td>
              <td className="py-1.5 pr-3 font-mono">internal_error</td>
              <td className="py-1.5">
                Kesalahan di sisi kami. Aman untuk dicoba lagi nanti; hubungi support bila terus
                berulang.
              </td>
            </tr>
          </tbody>
        </table>
      </Section>

      <Section title="Mengulang request dengan aman">
        <p>
          Jaringan yang buruk atau koneksi terputus adalah kejadian normal, bukan insiden. Kirim header{" "}
          <code className="font-mono">Idempotency-Key</code> berisi nilai unik Anda sendiri (mis. UUID) di
          setiap permintaan — bila permintaan yang sama diulang dengan key yang sama, Anda akan menerima{" "}
          <strong>lead yang sama persis</strong>, bukan duplikat dan bukan kesalahan. Key ini disimpan 48
          jam; setelahnya, key yang sama akan dianggap sebagai lead baru.
        </p>
      </Section>

      <Section title="Batas kecepatan">
        <p>Setiap response membawa empat header berikut:</p>
        <table className="w-full border-collapse text-[12.5px]">
          <tbody>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">X-RateLimit-Limit</td>
              <td className="py-1.5">Batas permintaan per menit untuk kunci Anda.</td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">X-RateLimit-Remaining</td>
              <td className="py-1.5">Sisa jatah di jendela saat ini.</td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">X-RateLimit-Reset</td>
              <td className="py-1.5">Waktu (Unix timestamp) jatah Anda kembali penuh.</td>
            </tr>
            <tr>
              <td className="py-1.5 pr-3 font-mono">Retry-After</td>
              <td className="py-1.5">Hanya muncul pada 429 — detik sampai boleh mencoba lagi.</td>
            </tr>
          </tbody>
        </table>
        <p className="text-muted-foreground">
          Lihat <code className="font-mono">X-RateLimit-Limit</code> di response Anda sendiri untuk angka
          pasti — batasnya bisa disesuaikan dari sisi kami tanpa mengubah halaman ini.
        </p>
      </Section>

      <Section title="Saat kunci dicabut">
        <p>
          Permintaan berikutnya dengan kunci yang sudah dicabut langsung ditolak dengan{" "}
          <code className="font-mono">401 invalid_api_key</code> — seketika, tanpa masa tenggang.
          Integrasi yang masih mengirim dengan kunci lama perlu diperbarui ke kunci yang baru.
        </p>
      </Section>

      <CreateAPIKeyDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => setRefreshKey((k) => k + 1)}
      />
    </div>
  );
}
