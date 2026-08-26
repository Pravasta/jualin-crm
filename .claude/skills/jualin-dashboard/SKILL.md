---
name: jualin-dashboard
description: Konvensi menulis kode frontend Next.js untuk Jualin CRM dashboard — struktur route, klien API, sesi, penanganan error, optimistic locking di form, shadcn/ui, dan pola testing. Gunakan setiap kali menulis, mengubah, atau mereview kode di crm_dashboard/.
---

# Jualin Dashboard — Konvensi Menulis Kode

> **Skill ini menjawab "bagaimana menulis kode di `crm_dashboard/`".**
> Untuk "apa yang harus dibangun dan kenapa", baca `docs/phases/03-owner-dashboard/{prd,td}.md`.
> Untuk sisi Go, pakai skill `jualin-backend`.
>
> Fondasi (proyek, klien API, sesi, lima layar auth) dibangun di **issue #31**. Catatan
> implementasinya — termasuk keputusan yang mudah salah dibaca dari kode saja — ada di
> `docs/phases/03-owner-dashboard/notes.md` bagian `## #31`.

---

## Stack

Next.js **App Router** · TypeScript · Tailwind **v4** · shadcn/ui (style `base-nova`, primitive `@base-ui/react`, ikon `lucide-react`) · **Vitest**

**Tanpa** state manager eksternal (Redux/Zustand/Jotai), **tanpa** data-fetching library (React Query/SWR), **tanpa** library i18n, **tanpa** library form (react-hook-form/zod). Bila salah satunya terasa dibutuhkan, itu sinyal untuk berhenti dan mendiskusikan — bukan menambahkannya.

> Alasan tidak ada library form: form di produk ini 2–4 field per layar. Alasan tidak ada i18n: keputusan **C1** — Bahasa Indonesia saja (`docs/phases/03-owner-dashboard/prd.md`).

---

## Struktur

```
crm_dashboard/                ← npm workspace sendiri, BUKAN bagian module Go
  src/
    app/
      (auth)/                 route group publik — TIDAK pernah memanggil GET /v1/me
        layout.tsx            kartu sempit di tengah
        login/ register/ verify-email/ forgot-password/ reset-password/
      (protected)/            route group terproteksi — SELALU di balik SessionGate
        layout.tsx            <SessionGate>{children}</SessionGate>
        page.tsx              home
      layout.tsx              root: lang="id", font, globals.css
      globals.css             token Tailwind v4 + shadcn (oklch)

    components/
      ui/                     shadcn/ui — DI-COPY ke repo (keputusan C3), boleh diubah
      field-error.tsx         pesan di bawah satu field
      form-error-banner.tsx   pesan global non-field

    lib/
      api-client.ts           apiFetch — credentials, CSRF, refresh single-flight
      api-types.ts            Envelope<T>, ApiError, ErrorDetail
      auth.ts                 pembungkus /v1/auth/* dan /v1/me
      auth-errors.ts          error.code → perilaku
      csrf.ts                 baca cookie csrf_token
      session-context.tsx     SessionGate + useSession()
      utils.ts                cn() dari shadcn

  vitest.config.mts           .mts, bukan .ts — lihat "Jebakan yang sudah pernah kena"
  components.json             config shadcn, registries kosong
```

Perintah dijalankan dari **`crm_dashboard/`**, bukan akar repository:

```
npm run dev · npm run typecheck · npm run lint · npm run test · npm run build
```

**Buat folder hanya saat layarnya dikerjakan.** Route kosong adalah utang yang terlihat seperti kemajuan.

---

## Klien API — satu-satunya jalan ke backend

`apiFetch` sudah menangani **empat** hal. Jangan menulis ulang salah satunya di call site:

```ts
import { apiFetch } from "@/lib/api-client";

const lead = await apiFetch<Lead>(`/v1/leads/${id}`);
await apiFetch(`/v1/leads/${id}/status`, {
  method: "PATCH",
  body: { status: "contacted", version },   // objek biasa — JSON.stringify otomatis
});
```

| Ditangani otomatis | Artinya |
|---|---|
| `credentials: 'include'` | Cookie sesi ikut terkirim di setiap request |
| Header `X-CSRF-Token` | Dipasang di **setiap non-GET**, dibaca dari cookie `csrf_token` |
| `Content-Type: application/json` | Saat ada `body` |
| **Refresh single-flight** | `401` → satu panggilan refresh dipakai bersama, request diulang **sekali** |

Gagal → melempar **`ApiError`** (`status`, `code`, `details`, `body`). Sukses → mengembalikan isi `data` saja.

### Yang dilarang

```ts
// ⛔ Menyentuh token — access_token & refresh_token HttpOnly, JavaScript TIDAK BISA membacanya
document.cookie.match(/access_token=/)
localStorage.setItem("token", …)          // Aturan #25, tidak pernah

// ⛔ fetch mentah ke API — kehilangan CSRF, credentials, DAN refresh single-flight
fetch(`${process.env.NEXT_PUBLIC_API_BASE_URL}/v1/leads`)

// ⛔ Memasang CSRF manual — apiFetch sudah melakukannya; di GET justru salah
headers: { "X-CSRF-Token": … }

// ⛔ Memanggil /v1/auth/refresh sendiri — merusak single-flight,
//    backend membacanya sebagai reuse attack lalu mencabut seluruh sesi
```

> **Kenapa single-flight bukan optimasi:** satu layar dengan enam widget yang semuanya menerima `401`
> tanpa single-flight memanggil refresh enam kali. Rotasi refresh token (issue #10) menganggap token
> yang sudah dirotasi sebagai **reuse attack** dan mencabut seluruh `family_id` — pengguna terlempar
> keluar, dan gejalanya terlihat seperti *"aplikasi mengeluarkan saya sendiri"*, bukan seperti bug
> klien. Kegagalan ini **hanya muncul di bawah konkurensi**.

### Endpoint list (`meta`)

`apiFetch` mengembalikan `data` saja dan **membuang `meta`**. Endpoint berpaginasi butuh `meta.total`.

**Tambahkan fungsi baru** (mis. `apiFetchList`) yang mengembalikan `{ data, meta }` — **jangan** ubah signature `apiFetch`, karena akan memaksa setiap pemanggil non-list menangani `meta` yang tidak mereka butuhkan.

> Jumlah total **selalu** dari `meta.total`, tidak pernah dari `data.length` — panjang array hanya
> mewakili halaman yang sedang terlihat.

---

## Sesi & proteksi route

Ditegakkan dengan **memanggil `GET /v1/me`**, bukan dengan memeriksa token — tidak bisa, token `HttpOnly`.

```tsx
// (protected)/layout.tsx — satu-satunya penjaga
export default function ProtectedLayout({ children }: { children: React.ReactNode }) {
  return <SessionGate>{children}</SessionGate>;
}

// Layar mana pun di bawahnya:
const session = useSession();   // dijamin sudah ada; melempar bila dipakai di luar (protected)
session.role                    // "owner" | "admin" | "manager" | "employee"
session.organization_name
```

**Tidak ada `middleware.ts`** yang membaca token. Layar baru cukup diletakkan di dalam `(protected)/` — tidak ada pengecekan tambahan yang perlu ditulis.

---

## Error — `code` menggerakkan perilaku, `message` ditampilkan apa adanya

Backend sudah mengirim `error.message` dalam **Bahasa Indonesia** sejak issue #9. **Jangan menerjemahkan ulang di klien** — dua sumber kebenaran untuk satu kalimat akan berbeda suatu saat, dan yang di layar belum tentu yang benar.

```ts
import { fieldErrorsFrom, globalMessage, organizationsFrom } from "@/lib/auth-errors";

try { … } catch (err) {
  const fields = fieldErrorsFrom(err);           // validation_failed → { email: "Wajib diisi." }
  if (Object.keys(fields).length) setFieldErrors(fields);
  else setError(globalMessage(err));             // sisanya → banner, message apa adanya
}
```

Empat kode butuh perlakuan khusus — **bukan** toast biasa:

| `code` | HTTP | Perilaku wajib |
|---|---|---|
| `validation_failed` | 400 | `details[]` tampil **di bawah field masing-masing** (`<FieldError>`), bukan sebagai pesan global |
| `version_conflict` | 409 | Tampilkan konflik, muat ulang dari `error.current`, **JANGAN PERNAH menimpa otomatis** |
| `organization_selection_required` | 409 | Tampilkan pemilih organization dari `error.organizations`, panggil ulang login |
| `membership_has_open_leads` | 409 | Dialog **memaksa memilih**: lepas assignment / pindahkan / batal — dengan `error.open_lead_count` |

> `membership_has_open_leads` **tidak boleh** disederhanakan jadi *"Yakin? [Ya]/[Batal]"*. Lead yang
> tetap ter-assign ke orang yang tidak bisa login lagi tidak muncul di daftar siapa pun **dan tidak
> tertangkap filter "belum ter-assign"** — karena secara teknis ia masih punya pemilik. Itu persis
> kegagalan senyap yang aturan itu dibangun untuk mencegah (freeze 2.3).

Aksi yang role-nya tidak izinkan **tidak ditawarkan di UI** — bukan ditawarkan lalu ditolak backend. Otorisasi sungguhan tetap di backend; UI hanya tidak berbohong tentang apa yang bisa dilakukan.

---

## `version` wajib bolak-balik lewat form

`leads` dan `tasks` memakai optimistic locking (Aturan #35).

```tsx
const [lead, setLead] = useState<Lead>();     // simpan versi yang DITERIMA saat membaca
await apiFetch(`/v1/leads/${id}`, {
  method: "PATCH",
  body: { name, version: lead.version },      // kirim kembali apa adanya
});
```

Form yang lupa membawanya bekerja **tepat sekali**, lalu gagal `409` di setiap penyimpanan berikutnya — gejala yang mudah salah dibaca sebagai "backend rusak". Bila `409` muncul saat pengembangan, **periksa form-nya lebih dulu**.

Setelah `version_conflict`, pakai `version` dari `error.current` — jangan menaikkan sendiri.

---

## shadcn/ui

```bash
npx shadcn@latest add table select dialog badge
```

Komponen **di-copy ke `src/components/ui/`** (keputusan C3) — bukan dependensi runtime. Boleh diubah langsung; itu memang tujuannya.

| Aturan | |
|---|---|
| Ambil komponen **saat dibutuhkan**, bukan borongan di muka | Aturan #27 |
| `components.json` → `registries: {}` **tetap kosong** | Tidak ada registry privat; menambahkannya = konfigurasi untuk kebutuhan yang belum ada |
| Tailwind **v4** — konfigurasi di `globals.css` (`@theme inline`), **tidak ada** `tailwind.config.js` | Jangan buat file itu |
| Warna lewat token (`bg-card`, `text-muted-foreground`, `border-input`) | Bukan `bg-zinc-50` mentah — token yang membuat perubahan tema jadi satu tempat |
| **Tanpa dark mode** | Di luar cakupan Phase 3; token `.dark` ada tetapi tidak dipakai |

---

## Bahasa & penamaan

- **Seluruh teks antarmuka: Bahasa Indonesia.** Termasuk label tombol, header tabel, dan *empty state*.
- **Kode, nama variabel, nama file: Inggris.**
- Field API tetap `snake_case` (`full_name`, `lead_number`) — jangan diubah jadi camelCase saat parsing; itu menciptakan dua nama untuk satu hal.

Ikuti `docs/product/glossary.md`. Yang paling sering keliru **di layar**:

| ⛔ Jangan | ✅ Pakai |
|---|---|
| "Workspace" | "Organization" |
| "Team" / "Tim saya" | "Semua lead" / "seluruh organisasi" — entity `Team` belum ada |
| `employee_id`, `member_id` | `membership_id` |
| Angka uang / revenue / nilai deal | — (Deal belum ada, jangan ditampilkan) |

Lead ditampilkan sebagai **nomor urut** (`#1024`), bukan UUID.

---

## Testing

| Jenis | Cara |
|---|---|
| Lapisan klien API | **Vitest** — mock `global.fetch`, uji perilaku yang gagal tanpa terlihat |
| Konkurensi (refresh single-flight) | **Wajib benar-benar paralel** — lihat di bawah |
| Pemetaan `error.code` → perilaku | Vitest |
| `version` ikut terkirim di form edit | Vitest |
| Test UI visual (snapshot, e2e penuh) | **Di luar cakupan Phase 3** (TD §9) — bentuk layar paling mungkin berubah setelah demo pertama |

**Test konkurensi harus benar-benar konkuren.** Test berurutan akan hijau meski logikanya salah total — persis jebakan yang sama seperti alokasi `lead_number` di backend issue #19. Polanya: tahan panggilan refresh dengan promise yang bisa di-resolve manual, biarkan **seluruh** pemanggil paralel mencapai titik single-flight, baru lepaskan.

```ts
const gate = deferred<Response>();                       // refresh ditahan
const calls = Array.from({ length: 6 }, () => apiFetch("/v1/widgets"));
await new Promise((r) => setTimeout(r, 0));              // biarkan keenam 401 mendarat
expect(refreshCallCount).toBe(1);                        // ← asersi yang sebenarnya
gate.resolve(okResponse);
```

---

## Jebakan yang sudah pernah kena

Tercatat supaya tidak didiagnosis dari nol dua kali (`notes.md` `## #31`):

| Jebakan | Yang benar |
|---|---|
| `LayoutProps<"/">` di layout | Pakai `{ children: React.ReactNode }`. Tipe generated bergantung pada `.next/types` yang belum ada di checkout bersih — `npm run typecheck` gagal di CI sebelum `npm run build` sempat jalan. |
| `vitest.config.ts` + `__dirname` | Berkasnya **`.mts`**, dan pakai `import.meta.dirname`. `package.json` tidak `"type": "module"`. |
| Test tidak terdeteksi | Vitest hanya memuat `src/**/*.test.ts` — **`.test.tsx` tidak ikut**. Ubah `vitest.config.mts` bila benar-benar butuh test komponen. |
| `useSearchParams()` membuat build gagal | Bungkus komponennya dengan `<Suspense>`; pola yang sudah dipakai `verify-email/` dan `reset-password/`. |
| Warning ESLint `no-location-assign-relative-destination` | Hanya dikecualikan di `api-client.ts` (di luar React tree, tidak punya `useRouter`). Di dalam komponen, pakai `useRouter().push()`. |

---

## Sebelum menyatakan sebuah layar selesai

- [ ] `npm run typecheck` bersih
- [ ] `npm run lint` bersih — **0 error, 0 warning**
- [ ] `npm run test` lolos; test konkurensi diulang beberapa kali (bukan sekali lalu diasumsikan stabil)
- [ ] `npm run build` sukses
- [ ] Seluruh teks Bahasa Indonesia; `error.message` backend tampil **apa adanya**
- [ ] `validation_failed` tampil **di bawah field**, bukan sebagai banner global
- [ ] Form edit `lead`/`task` membawa `version`, dan `409` ditangani tanpa menimpa otomatis
- [ ] Keadaan kosong membedakan **"belum ada data"** dari **"tidak cocok filter"**
- [ ] Keadaan memuat dan gagal memuat punya tampilan
- [ ] Aksi yang role-nya tidak izinkan tidak ditawarkan
- [ ] Tidak ada `fetch` mentah, tidak ada akses token, tidak ada CSRF manual
- [ ] Jumlah total dari `meta.total`, bukan `data.length`
- [ ] Diverifikasi terhadap `crm_be` sungguhan (`docker compose up` + `make migrate-up`), bukan hanya mock
- [ ] `docs/phases/03-owner-dashboard/notes.md` ditulis
- [ ] `docs/STATUS.md` diperbarui

---

## Alur kerja

Sama seperti seluruh repository ini: **1 issue = 1 session = 1 PR** (`docs/workflow.md`, ADR-008).
Dokumentasi ditulis **di dalam PR yang sama**, bukan menyusul. PR memakai `Refs #N`, **bukan** `Closes #N`.

Desain visual datang dari `docs/phases/03-owner-dashboard/design-brief.md`. Bila hasil desain
bertentangan dengan `prd.md`/`td.md`, **laporkan** — jangan diam-diam mengikuti salah satunya.
