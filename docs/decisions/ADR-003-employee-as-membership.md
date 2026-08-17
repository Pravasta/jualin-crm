# ADR-003 — Employee adalah Membership, bukan Entity

> **Status:** ✅ Accepted — 17 Agustus 2026
> **Terkait:** [ADR-007](./ADR-007-user-organization-cardinality.md)

## Konteks

Dokumen produk awal memperlakukan `User` dan `Employee` sebagai dua hal berbeda — `Employees` digambarkan sebagai anak Organization, ada usulan modul `internal/employee/`, dan assignment lead ditulis sebagai `lead.assigned_to = employee_id`.

Pada saat yang sama, `Employee` juga merupakan salah satu **role** (owner, admin, manager, employee).

Dua pembacaan ini tidak bisa hidup berdampingan.

## Keputusan

**Tidak ada tabel `employees`.**

```
User ──< Membership >── Organization
              │
              └── role: owner | admin | manager | employee
```

| Konsep | Definisi |
|---|---|
| **User** | Identitas login. Global, lintas organization. |
| **Membership** | Keanggotaan user pada satu organization + role. **Inilah "Employee".** |
| **Employee** | Membership dengan `role = 'employee'` |

**Assignment dan aktor selalu menunjuk `membership_id`** — bukan `user_id`, bukan `employee_id`.

## Alasan

### 1. Menghindari identity ambiguity permanen

Bila Employee dijadikan tabel terpisah, tiga pertanyaan berikut tidak akan punya jawaban bersih:

- Owner UMKM yang ikut menangani lead sendiri — perlu record Employee?
- Employee dipromosikan jadi Manager — recordnya pindah tabel? Lead lamanya ikut?
- Manager yang memegang lead sendiri — dia User atau Employee?

Setiap jawaban menghasilkan pengecualian, dan pengecualian itu menyebar ke setiap query.

### 2. `membership_id` membuat isolasi tenant menjadi struktural

Ini alasan teknis yang lebih penting.

Satu `user_id` bisa berada di dua organization. Bila lead menunjuk `user_id`, isolasi tenant bergantung pada disiplin query di setiap tempat.

`membership_id` **terikat pada satu organization secara definisi**, sehingga bisa ditegakkan database lewat composite FK:

```sql
FOREIGN KEY (assigned_to_membership_id, organization_id)
  REFERENCES memberships (id, organization_id)
```

Dengan `user_id`, constraint ini tidak mungkin dibuat — dan konsekuensinya karyawan Org B bisa melihat lead Org A di aplikasi mobile-nya.

## Konsekuensi

**Positif:** satu model identitas · promosi/demosi hanya mengubah satu kolom · isolasi tenant ditegakkan database · atribusi historis tetap utuh saat employee nonaktif (soft delete membership menjaga FK)

**Negatif:** query "daftar employee" selalu memfilter `role = 'employee'` · nama "membership" kurang intuitif dibanding "employee"

**Mitigasi:** UI tetap memakai istilah yang dipahami pengguna. Di kode dan database, hanya `membership`. Daftar istilah terlarang ada di `docs/product/glossary.md`.

## Bila nanti butuh atribut HR

NIP, departemen, atasan, foto → tabel `employee_profile` **1:1 ke membership**, bukan menggantinya.

Itu juga pintu masuk alami ke Jualin HRIS — sebagai produk terpisah, bukan di repository ini.

## Konsekuensi yang wajib ditegakkan

| # | Aturan |
|---|---|
| 1 | Jangan pernah membuat tabel `employees` |
| 2 | Jangan pernah memakai identifier `employee_id`, `staff_id`, atau `member_id` |
| 3 | Assignment, aktor Activity, aktor Audit log, dan penerima Notification **semuanya** menunjuk `membership_id` |
| 4 | Menonaktifkan employee = soft delete membership, **bukan** hapus user |
