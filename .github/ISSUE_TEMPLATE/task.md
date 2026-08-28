---
name: Task
about: Satu unit pekerjaan — 1 issue = 1 session = 1 PR
title: ''
labels: ''
assignees: ''
---

## Cakupan

<!-- Apa yang dikerjakan. Ringkas, 2-4 kalimat. -->

## Checklist

<!-- Pecahan pekerjaan DI DALAM issue ini. Jangan dipecah jadi issue terpisah. -->

- [ ]
- [ ]
- [ ]

## Acceptance Criteria

<!-- Kondisi terukur. Bukan "selesai", tapi "X menghasilkan Y". -->

- [ ]
- [ ]

## Tidak termasuk

<!-- Yang SENGAJA tidak dikerjakan di sini, dan ke mana ia pergi. -->

## Referensi

- Phase: `docs/phases/<NN>-<slug>/`
- TD: `docs/phases/<NN>-<slug>/td.md` §
- Aturan terkait: `docs/architecture/freeze.md` bagian 5 #

## Definition of Done

- [ ] Test lolos, termasuk **test isolasi tenant** bila menyentuh tenant boundary
- [ ] Otorisasi diuji per role bila menambah endpoint
- [ ] `docs/phases/<NN>-<slug>/notes.md` diperbarui
- [ ] `docs/STATUS.md` diperbarui bila ada utang teknis atau phase selesai
- [ ] `docs/issues/<NNN>-<slug>.md` ditambah/diperbarui bila ada temuan di luar cakupan issue ini yang perlu ditinjau ulang saat penutup phase (skill `jualin-issue-log`) — kosongkan bila tidak ada
- [ ] PR dibuka dengan `Refs #<issue>` — **bukan** `Closes`
