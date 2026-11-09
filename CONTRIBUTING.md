# Panduan Kontribusi

Dokumen ini singkat dan sengaja tegas, supaya kode dari lima orang tetap seragam.
Baca juga `README.md` (struktur proyek), `docs/spec-perhitungan.md` (aturan hitung),
dan PRD (kebutuhan F-1 sampai F-27 dan L-1 sampai L-4).

## 1. Menyiapkan lingkungan

1. Pasang Go sesuai versi di `go.mod` dan MySQL 8.0.16 atau lebih baru.
2. `git clone`, lalu jalankan `make test`. Semua harus hijau sebelum mulai bekerja.
3. Jalankan `make run` dan buka `http://localhost:8080`.

## 2. Alur kerja Git

- **Jangan push langsung ke `main`.** Semua perubahan lewat Pull Request (PR).
- Satu tugas (issue) = satu branch = satu PR. Nama branch: `fitur/anggota-daftar`,
  `perbaikan/denda-hari-terakhir`, `dokumen/readme`.
- Ambil kode terbaru sebelum mulai dan sebelum membuka PR:
  `git checkout main && git pull`, lalu `git rebase main` di branch Anda.
- Pesan commit singkat dan jelas dan mengikuti git convention https://www.conventionalcommits.org/en/v1.0.0/.
- Satu PR butuh minimal satu persetujuan dan CI hijau sebelum di-merge.
- Yang merge adalah pembuat PR, setelah disetujui.

## 3. Siapa memegang apa

| Fitur | Pemilik | Berkas utama |
| --- | --- | --- |
| Fondasi, review | Ketua tim | `cmd/`, `internal/config`, `internal/middleware`, `migrations/` |
| Anggota dan simpanan | Dev A | `*/anggota.go`, `templates/anggota_*.html` |
| Pinjaman, jadwal, pelunasan | Dev B | `*/pinjaman.go`, `templates/pinjaman_*.html` |
| Pembayaran dan pembatalan | Dev C | `*/pembayaran.go`, `templates/pembayaran_*.html` |
| Laporan dan ekspor | Dev D | `*/laporan.go`, `templates/laporan_*.html` |

Setiap fitur memakai **satu berkas per lapisan dengan nama fitur**: `repository/anggota.go`,
`service/anggota.go`, `handler/anggota.go`, `router/anggota.go`. Dengan begitu dua orang jarang
mengubah berkas yang sama. Bila Anda harus mengubah berkas milik fitur lain, bicarakan dulu.

**Berkas bersama** (`router/router.go`, `cmd/koperasi/main.go`, `templates/layout.html`) hanya diubah
lewat PR kecil tersendiri yang disetujui ketua tim.

**Perhitungan uang** (`service/batas.go`, `jadwal.go`, `denda.go`, `alokasi.go`, `pelunasan.go`) dan
`migrations/` hanya diubah dengan persetujuan ketua tim.

## 4. Aturan kode

1. **Tidak ada aturan bisnis di handler.** Handler membaca form, memanggil `service`, lalu menampilkan
   hasil. Semua aturan (batas pinjaman, denda, validasi) ada di `service`.
2. **Uang selalu `int64` rupiah.** Dilarang memakai `float32` atau `float64` untuk uang.
3. **Tanggal bisnis memakai tanggal saja** (zona waktu Asia/Jakarta), terpisah dari waktu pencatatan.
4. **Fungsi perhitungan murni** (di `service`) tidak boleh mengimpor `database/sql` atau `repository`,
   dan tidak boleh memanggil `time.Now()`. Tanggal "hari ini" diberikan sebagai argumen.
5. **Transaksi tidak dihapus.** Koreksi dilakukan dengan membatalkan lalu memasukkan ulang
   (lihat PRD 4.6). Jangan menulis `DELETE` atau `UPDATE` atas nominal.
6. **Setiap perubahan data menulis log audit.**
7. **Setiap aturan di `service` punya tes tabel** (table-driven) yang mencakup kasus batas.
8. **Penamaan:** istilah domain Bahasa Indonesia (`Anggota`, `Angsuran`, `HitungDenda`),
   istilah teknis Bahasa Inggris (`Handler`, `Repository`, `Config`). Antarmuka berbahasa Indonesia.
9. **Kesalahan tidak diabaikan.** Jangan `_ = err`. Bungkus dengan konteks: `fmt.Errorf("simpan anggota: %w", err)`.
10. **Format dan vet** harus bersih: `make fmt` sebelum commit, `make test` sebelum push.

## 5. Migrasi basis data

- Perubahan skema = berkas migrasi baru bernomor urut: `000002_tambah_xxx.up.sql` dan `.down.sql`.
- **Jangan mengubah migrasi yang sudah di-merge.**
- Nomor urut diberikan ketua tim supaya tidak bentrok. Minta nomor sebelum membuat berkasnya.

## 6. Menulis fitur baru: ikuti fitur contoh

Fitur contoh (pendaftaran anggota) disiapkan ketua tim sebagai acuan. Salin polanya: repository, service
dengan tes, handler, template, dan log audit. Bila ragu bagaimana menulisnya, lihat fitur contoh sebelum
bertanya atau membuat pola baru.

## 7. Review

Pemeriksa membaca PR dengan daftar di `.github/pull_request_template.md`. Review yang baik:

- Jalankan branch-nya bila perubahannya terlihat.
- Komentar spesifik dan sopan ("bagaimana bila jumlahnya 0?") bukan sekadar "kurang bagus".
- Beda pendapat soal gaya diputuskan ketua tim, lalu dicatat di dokumen ini agar tidak diulang.
- Semua anggota me-review bergiliran. Itu juga cara belajar tercepat.

## 8. Bila macet

Macet lebih dari 30 menit: tanyakan di grup dengan menyertakan pesan galat, kode yang relevan,
dan apa yang sudah dicoba. Lebih baik bertanya cepat daripada diam sehari.
