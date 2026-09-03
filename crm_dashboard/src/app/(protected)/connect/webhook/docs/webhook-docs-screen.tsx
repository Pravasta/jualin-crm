"use client";

// The receiver-facing reference for outbound webhooks (issue #104, Phase
// 7 close). Its reader is a developer on the CUSTOMER's side who has to
// verify our signature in their own code — the one audience that never
// sees the dashboard otherwise. Lives here, next to the management
// screen, following /connect/api/docs (#49).
//
// Unlike that page, this one CAN show genuinely working code: verifying a
// signature needs the receiver's own saved secret, not a credential of
// ours (see lib/webhook-docs.ts). So acceptance criterion #2 — "contoh
// disalin, dijalankan, benar-benar memvalidasi, lalu payload diubah satu
// byte dan ditolak" — is something this page actually delivers, not
// gestures at.
//
// Role-gated to Owner/Admin (canManageWebhooks), same as every other
// webhook screen: a Manager who types this URL gets "tidak tersedia" and
// makes zero API calls (the gate sits above the useEffect).
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { FormErrorBanner } from "@/components/form-error-banner";
import { canManageWebhooks } from "@/lib/webhook-permissions";
import {
  listWebhookEndpoints,
  WEBHOOK_EVENT_LABELS,
  type WebhookEndpoint,
} from "@/lib/webhooks";
import {
  SAMPLE_PAYLOAD,
  SAMPLE_STATUS_CHANGED_EXTRA,
  SIGNATURE_HEADER,
  SIGNATURE_TOLERANCE_SECONDS,
  WEBHOOK_DOC_EXAMPLES,
} from "@/lib/webhook-docs";
import { globalMessage } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";

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

function CodeBlock({ children }: { children: string }) {
  return (
    <pre className="overflow-x-auto rounded-md border border-border bg-muted/40 p-2.5 font-mono text-[11.5px] whitespace-pre">
      {children}
    </pre>
  );
}

export function WebhookDocsScreen() {
  const session = useSession();
  const router = useRouter();
  const canManage = canManageWebhooks(session.role);

  // Active endpoints only — a deactivated one's prefix would suggest an
  // example that will never receive a delivery.
  const [activeEndpoints, setActiveEndpoints] = useState<WebhookEndpoint[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [selectedId, setSelectedId] = useState<string>("");
  const [lang, setLang] = useState(WEBHOOK_DOC_EXAMPLES[0].language);

  useEffect(() => {
    if (!canManage) return;
    const controller = new AbortController();
    listWebhookEndpoints(controller.signal)
      .then((endpoints) => {
        const active = endpoints.filter((e) => e.is_active);
        setActiveEndpoints(active);
        setSelectedId((current) => current || active[0]?.id || "");
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
        Dokumentasi webhook tidak tersedia untuk role Anda.
      </p>
    );
  }

  const selected = activeEndpoints.find((e) => e.id === selectedId) ?? null;
  const example = WEBHOOK_DOC_EXAMPLES.find((e) => e.language === lang) ?? WEBHOOK_DOC_EXAMPLES[0];

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-[15px] font-semibold">Dokumentasi webhook</h1>
        <Button variant="outline" onClick={() => router.push("/connect/webhook")}>
          ← Kembali ke Webhook
        </Button>
      </div>

      <p className="text-[13px] text-muted-foreground">
        Halaman ini untuk developer di sisi <strong>penerima</strong> — sistem yang menerima kiriman
        dari Jualin dan perlu memastikan setiap kiriman benar-benar berasal dari kami.
      </p>

      <FormErrorBanner message={error} />

      <Section title="Endpoint Anda">
        {!loaded ? (
          <p className="text-muted-foreground">Memuat…</p>
        ) : activeEndpoints.length === 0 ? (
          <p className="text-muted-foreground">
            Anda belum punya endpoint aktif. Tambahkan satu di halaman Webhook — saat dibuat, Jualin
            menampilkan <span className="font-mono">signing secret</span> satu kali. Secret itulah
            yang dipakai contoh di bawah.
          </p>
        ) : (
          <div className="flex flex-col gap-1.5">
            <label className="text-[12.5px] text-muted-foreground" htmlFor="docs-endpoint-select">
              Contoh di bawah mengacu ke endpoint:
            </label>
            <select
              id="docs-endpoint-select"
              value={selectedId}
              onChange={(e) => setSelectedId(e.target.value)}
              className="h-8.5 w-full rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              {activeEndpoints.map((endpoint) => (
                <option key={endpoint.id} value={endpoint.id}>
                  {endpoint.url} ({endpoint.secret_prefix}…)
                </option>
              ))}
            </select>
            {selected && (
              <p className="text-[12.5px] text-muted-foreground">
                Melanggan:{" "}
                {selected.events.map((ev) => WEBHOOK_EVENT_LABELS[ev] ?? ev).join(", ")}.
              </p>
            )}
          </div>
        )}
      </Section>

      <Section title="Bentuk payload">
        <p>
          Setiap kiriman adalah <span className="font-mono">POST</span> dengan body JSON. Isinya
          dibekukan saat event terjadi — kalau lead berubah tiga kali dalam lima menit, Anda menerima
          tiga kiriman dengan isi berbeda, bukan tiga salinan keadaan terakhir.
        </p>
        <CodeBlock>{SAMPLE_PAYLOAD}</CodeBlock>
        <p className="text-muted-foreground">
          <span className="font-mono">delivery_id</span> tetap sama di seluruh percobaan ulang untuk
          satu kiriman — pakai itu untuk deduplikasi (lihat bagian pengiriman ganda di bawah).{" "}
          <span className="font-mono">occurred_at</span> adalah waktu event, bukan waktu kiriman;
          sebuah retry enam jam kemudian tetap melaporkan waktu aslinya.
        </p>
        <p>
          Event <span className="font-mono">lead.status_changed</span> menambahkan satu kunci{" "}
          <span className="font-mono">changes</span>:
        </p>
        <CodeBlock>{SAMPLE_STATUS_CHANGED_EXTRA}</CodeBlock>
      </Section>

      <Section title="Header">
        <table className="w-full border-collapse text-[12.5px]">
          <tbody>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">{SIGNATURE_HEADER}</td>
              <td className="py-1.5">
                <span className="font-mono">t=&lt;unix detik&gt;,v1=&lt;hex HMAC-SHA256&gt;</span> —
                bukti kiriman berasal dari Jualin. Cara memverifikasinya di bawah.
              </td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">Content-Type</td>
              <td className="py-1.5 font-mono">application/json</td>
            </tr>
            <tr>
              <td className="py-1.5 pr-3 font-mono">User-Agent</td>
              <td className="py-1.5 font-mono">Jualin-Webhook/1</td>
            </tr>
          </tbody>
        </table>
      </Section>

      <Section title="Memverifikasi signature">
        <p>Tiga langkah, dan urutannya penting:</p>
        <ol className="ml-4 list-decimal space-y-1 text-[12.5px]">
          <li>
            Baca body <strong>mentah</strong> — byte apa adanya, <strong>sebelum</strong> di-parse
            JSON. Mem-parse lalu menyusun ulang mengubah urutan kunci dan spasi, dan signature-nya
            langsung tidak cocok tanpa error di sisi mana pun.
          </li>
          <li>
            Ambil <span className="font-mono">t</span> dan <span className="font-mono">v1</span> dari
            header. Hitung <span className="font-mono">HMAC-SHA256(secret, &quot;&lt;t&gt;.&quot; + body mentah)</span>{" "}
            dalam hex, bandingkan dengan <span className="font-mono">v1</span> memakai perbandingan{" "}
            <em>constant-time</em>.
          </li>
          <li>
            Tolak kalau selisih <span className="font-mono">t</span> dengan waktu sekarang lebih dari{" "}
            <strong>{SIGNATURE_TOLERANCE_SECONDS / 60} menit</strong>. <span className="font-mono">t</span>{" "}
            ikut ditandatangani, jadi ia tidak bisa diubah tanpa merusak <span className="font-mono">v1</span>.
          </li>
        </ol>

        <div className="mt-1 flex gap-1.5">
          {WEBHOOK_DOC_EXAMPLES.map((ex) => (
            <button
              key={ex.language}
              type="button"
              onClick={() => setLang(ex.language)}
              className={
                ex.language === lang
                  ? "rounded-md border border-input bg-muted px-2.5 py-1 text-[12.5px] font-medium"
                  : "rounded-md border border-transparent px-2.5 py-1 text-[12.5px] text-muted-foreground hover:bg-muted/50"
              }
            >
              {ex.language}
            </button>
          ))}
        </div>
        <p className="text-[12px] text-muted-foreground">{example.filename}</p>
        <CodeBlock>{example.code}</CodeBlock>
        <p className="text-muted-foreground">
          Contoh ini langsung bisa dijalankan — isi <span className="font-mono">WEBHOOK_SIGNING_SECRET</span>{" "}
          dengan secret yang Anda salin saat membuat endpoint. Jualin tidak menyimpan raw secret Anda;
          begitu dialog pembuatan ditutup, kami sendiri pun tidak bisa menampilkannya lagi.
        </p>
      </Section>

      <Section title="Kebijakan percobaan ulang">
        <table className="w-full border-collapse text-[12.5px]">
          <thead>
            <tr className="border-b border-border text-left text-muted-foreground">
              <th className="py-1.5 pr-3 font-medium">Respons endpoint Anda</th>
              <th className="py-1.5 font-medium">Yang Jualin lakukan</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">2xx</td>
              <td className="py-1.5">Dianggap berhasil, selesai.</td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">429</td>
              <td className="py-1.5">Dicoba ulang — Anda yang meminta kami melambat.</td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">4xx lain</td>
              <td className="py-1.5">
                <strong>Tidak</strong> dicoba ulang. &quot;Permintaan salah&quot; tidak berubah dengan
                diulang — perbaiki endpoint, lalu kirim ulang dari halaman riwayat.
              </td>
            </tr>
            <tr className="border-b border-border/60">
              <td className="py-1.5 pr-3 font-mono">3xx</td>
              <td className="py-1.5">
                Gagal permanen. Redirect <strong>tidak pernah</strong> diikuti — daftarkan URL tujuan
                akhir langsung.
              </td>
            </tr>
            <tr>
              <td className="py-1.5 pr-3 font-mono">5xx / timeout / koneksi gagal</td>
              <td className="py-1.5">
                Dicoba ulang hingga 5 kali setelah kiriman pertama (maksimal 6 panggilan), dengan jeda
                menaik 1 menit → 5 menit → 30 menit → 2 jam → 6 jam. Setelah itu berstatus gagal
                permanen.
              </td>
            </tr>
          </tbody>
        </table>
        <p className="text-muted-foreground">
          Angka jeda dan jumlah percobaan di atas adalah default konservatif, bukan hasil pengukuran —
          bisa berubah dari sisi kami tanpa mengubah halaman ini.
        </p>
      </Section>

      <Section title="Satu kiriman bisa datang lebih dari sekali">
        <p>
          Jualin menjamin <em>at-least-once</em>, bukan <em>exactly-once</em>: bila proses kami
          terhenti tepat setelah mengirim tapi sebelum mencatat hasilnya, kiriman yang sama akan
          dikirim lagi. Karena itu setiap payload membawa <span className="font-mono">delivery_id</span>{" "}
          yang stabil lintas percobaan — simpan yang sudah Anda proses, dan abaikan yang berulang.
        </p>
      </Section>

      <Section title="Kalau pengiriman gagal saat disimpan">
        <p>
          Saat Anda mendaftarkan atau mengubah URL, Jualin menolaknya dengan{" "}
          <span className="font-mono">400 webhook_url_not_allowed</span> bila URL menunjuk alamat
          privat, loopback, atau link-local, bila skemanya bukan <span className="font-mono">http(s)</span>,
          atau bila DNS-nya tidak bisa diresolusi. Pesannya sengaja tidak membedakan alasan-alasan itu.
          Pakai URL publik yang bisa dijangkau dari internet.
        </p>
      </Section>

      <Section title="Signing secret Anda">
        <p>
          Ditampilkan <strong>sekali</strong>, saat endpoint dibuat. Database kami menyimpannya dalam
          bentuk terenkripsi, bukan raw — tapi begitu dialog pembuatan ditutup, tidak ada layar yang
          menampilkannya lagi. Kalau hilang, hapus endpoint dan buat baru; secret baru akan terbit.
        </p>
        {selected && (
          <p className="text-muted-foreground">
            Endpoint terpilih dimulai dengan{" "}
            <span className="font-mono">{selected.secret_prefix}…</span> — cukup untuk memastikan Anda
            memakai secret yang benar, tidak cukup untuk apa pun yang lain.
          </p>
        )}
      </Section>
    </div>
  );
}
