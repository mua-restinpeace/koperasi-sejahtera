# Koperasi Karyawan Sejahtera

Aplikasi web untuk mencatat simpanan dan pinjaman anggota koperasi karyawan: pendaftaran anggota,
simpanan pokok/wajib/sukarela, pinjaman dengan jadwal angsuran, denda, pelunasan dipercepat, dan laporan.
Dipakai oleh pengurus (login), bukan oleh anggota.

Dibuat dengan Go (hanya `net/http` dari pustaka standar + `html/template`) dan MySQL 8.

> **Status:** cabang `develop`. Sudah ada: login, inti perhitungan (jadwal, denda, alokasi, pelunasan, dengan tes),
> dan fitur **Anggota** lengkap (daftar, cari/filter, detail, ubah, keluar). Fitur **Anggota** adalah
> **contoh acuan**: fitur lain meniru polanya (lihat [Cara kerja kode](#cara-kerja-kode-ikuti-fitur-anggota)).

## Dokumen penting

| Dokumen | Isi |
| --- | --- |
| [`docs/spec-perhitungan.md`](docs/spec-perhitungan.md) | Aturan hitung uang. Setiap angka di bagian 6 adalah tes. |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Aturan kode dan alur kerja tim. |
| [`docs/setup-lokal.md`](docs/setup-lokal.md) | Catatan setup yang lebih singkat. |
| PRD | Kebutuhan fitur F-1 s.d. F-27 dan laporan L-1 s.d. L-4 (di luar repo, minta ke ketua tim). |

---

## Setup di laptop sendiri

### 1. Yang harus terpasang

| Alat | Versi | Catatan |
| --- | --- | --- |
| Go | sesuai baris `go 1.27.0` di [`go.mod`](go.mod) | Cek dengan `go version`. |
| MySQL | **8.0.16 atau lebih baru** | Versi lama tidak menegakkan `CHECK` constraint, skema bergantung padanya. |
| Klien `mysql` | bawaan MySQL | Untuk memuat data contoh. |
| [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) | v4, dengan driver mysql | Pasang sekali: `go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
| Git | | |

Pastikan folder `$(go env GOPATH)/bin` ada di `PATH` supaya perintah `migrate` ditemukan.

### 2. Ambil kode dan dependensi

```sh
git clone https://github.com/mua-restinpeace/koperasi-sejahtera.git
cd koperasi-sejahtera
git checkout develop
go mod download
```

### 3. Buat database

Dua database: satu untuk dijalankan, satu khusus tes (namanya **wajib** berakhiran `_test`).
Jalankan di klien MySQL sebagai `root`:

```sql
CREATE DATABASE koperasi      CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE koperasi_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'koperasi'@'%' IDENTIFIED BY 'koperasi';
GRANT ALL ON koperasi.*      TO 'koperasi'@'%';
GRANT ALL ON koperasi_test.* TO 'koperasi'@'%';
```

> Pengguna `koperasi` / kata sandi `koperasi` hanya untuk laptop sendiri.

### 4. Isi konfigurasi `.env`

```sh
cp .env.example .env
```

Buka `.env` dan isi **hanya** `KOPERASI_DSN`. Variabel lain boleh dibiarkan kosong; nilai bawaannya ada di
[`internal/config/config.go`](internal/config/config.go) (simpanan pokok Rp500.000, wajib Rp100.000, jasa 1%, dan seterusnya).

```env
KOPERASI_DSN=koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi?parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27
```

- `parseTime=true` **wajib**; aplikasi menolak start tanpanya.
- `loc=UTC` dan `time_zone=...` membuat semua cap waktu konsisten UTC. Tanggal bisnis memakai `DATE` dan zona Asia/Jakarta.
- `.env` sudah ada di `.gitignore`. **Jangan di-commit.**
- `.env` dibaca dari folder tempat Anda menjalankan perintah, jadi jalankan aplikasi dari akar repo.
- Lewat HTTP biasa (lokal) biarkan `KOPERASI_COOKIE_SECURE` kosong. Di server HTTPS set `true`.

### 5. Buat tabel (migrasi)

Dari akar repo:

```sh
migrate -path migrations -database "mysql://koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi?multiStatements=true" up
```

Untuk membatalkan satu migrasi terakhir, ganti `up` dengan `down 1`.
`multiStatements=true` diperlukan karena satu berkas migrasi berisi banyak perintah.

### 6. Isi data contoh (disarankan)

```sh
mysql -u koperasi -p koperasi -e "source migrations/000003_dummy_data_koperasi.sql"
```

(Kata sandi: `koperasi`.) Berkas ini sengaja **tidak** ikut dijalankan `migrate` karena namanya tanpa `.up.`.

> **Awas:** berkas ini diawali `TRUNCATE` pada **semua tabel**. Jangan dijalankan pada database yang berisi data
> yang ingin Anda simpan. Boleh dijalankan ulang untuk mengembalikan data contoh ke keadaan awal.

Isinya:

| Data | Keterangan |
| --- | --- |
| Pengguna `admin` | **Satu-satunya akun yang bisa login.** Username `admin`, kata sandi `adminpassword99`. |
| Pengguna `pengurus1`, `pengurus.nonaktif` | Kata sandinya masih placeholder, **belum bisa login**. |
| 6 anggota `KKS-0001` s.d. `KKS-0006` | 4 aktif, 2 sudah keluar. |
| `KKS-0001` Andi Pratama | Punya **pinjaman aktif**, jadi **tidak bisa dikeluarkan**. Berguna untuk mencoba penolakan. |
| `KKS-0002`, `KKS-0004`, `KKS-0006` | Aktif tanpa pinjaman aktif, bisa dikeluarkan. |

> Akun `admin` hanya untuk pengembangan. **Jangan** dipakai atau dibiarkan ada di server sungguhan.
> Angka pinjaman di data contoh hanya untuk mengisi tampilan; belum mengikuti
> [`docs/spec-perhitungan.md`](docs/spec-perhitungan.md), jadi jangan dipakai untuk memeriksa perhitungan.

### 7. Jalankan

```sh
go run ./cmd/koperasi
```

Buka <http://localhost:8080> dan login sebagai `admin`. Cek cepat tanpa login: <http://localhost:8080/healthz>.
Hentikan dengan `Ctrl+C` (server dimatikan dengan rapi). Port bisa diganti lewat `KOPERASI_ALAMAT` di `.env`, misalnya `:9090`.

---

## Perintah sehari-hari

Semua dijalankan dari akar repo. Tidak perlu `make`; berkas [`Makefile`](Makefile) hanya merangkum perintah yang sama.

| Tujuan | Perintah |
| --- | --- |
| Jalankan aplikasi | `go run ./cmd/koperasi` |
| Build ke `bin/koperasi` | `go build -o bin/koperasi ./cmd/koperasi` |
| Cek format (tampilkan berkas yang belum rapi) | `gofmt -l .` |
| Rapikan format | `gofmt -w .` |
| Analisis statis | `go vet ./...` |
| Semua tes unit | `go test ./...` |
| Semua tes dengan pendeteksi race (seperti CI) | `go test -race ./...` |
| Satu paket saja | `go test ./internal/service/` |
| Satu tes saja, rinci | `go test ./internal/service/ -run TestAnggota_Update -v` |
| Terapkan migrasi | `migrate -path migrations -database "<URL>" up` |
| Batalkan 1 migrasi | `migrate -path migrations -database "<URL>" down 1` |

`<URL>` adalah `mysql://koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi?multiStatements=true`.

**Sebelum push** jalankan ketiganya; CI menjalankan hal yang sama dan akan merah bila salah satunya gagal:

```sh
gofmt -l .          # tidak boleh mencetak apa pun
go vet ./...
go test -race ./...
```

Tes unit **tidak memerlukan database**: service diuji dengan repository tiruan (`fake_test.go`), dan
handler/router diuji lewat `httptest` dengan service tiruan. Target `make test-integrasi` dan CI untuk
tes dengan MySQL sungguhan sudah disiapkan, tetapi **belum ada berkas tes integrasi** (tag `integration`).

---

## Struktur proyek

```
cmd/koperasi/main.go         titik masuk: membaca config, membuat objek, menyambungkannya, menjalankan server
internal/config/             nilai koperasi (simpanan, jasa, denda, tenor) dan pengaturan server dari .env
internal/model/              struct data polos (Anggota, TransaksiSimpanan, Pinjaman, ...)
internal/repository/         SQL. Satu berkas per tabel/fitur. Tanpa aturan bisnis.
internal/service/            aturan bisnis, validasi, transaksi, log audit. Tes memakai repository tiruan.
internal/handler/            membaca form, memanggil service, menampilkan template atau mengalihkan
internal/router/router.go    daftar URL -> handler
internal/middleware/         login, CSRF, sesi, logging, header keamanan
internal/helper/             fungsi MURNI: jadwal, denda, alokasi, pelunasan, hash kata sandi, format rupiah
internal/database/           koneksi MySQL, tipe DBTX, pembantu transaksi
templates/                   layout/ (kerangka), partials/ (header, sidebar), pages/ (satu berkas per halaman)
static/css/app.css           gaya
migrations/                  skema basis data (berkas bernomor urut) dan data contoh
```

---

## Cara kerja kode: ikuti fitur Anggota

Satu permintaan melewati lapisan yang sama setiap kali. Contoh: pengurus menekan **Keluarkan Anggota**.

```
Browser  POST /anggota/keluar/5
   │
   ▼
router/router.go        r.Protected("POST /anggota/keluar/{id}", r.h.AnggotaResign)
   │                    Protected = wajib login + token CSRF diperiksa (untuk method selain GET)
   ▼
handler/anggota_handler.go   AnggotaResign
   │                    baca {id} -> panggil service -> pilih tampilan dari hasilnya
   ▼
service/anggota_service.go   Resign
   │                    cek login -> mulai transaksi -> aturan bisnis -> catat audit -> commit
   ▼
repository/*.go         Saldo, Create (penarikan), Deactivate: SQL saja
   ▼
MySQL
```

Aturan tiap lapisan (supaya kode semua orang seragam):

| Lapisan | Berkas contoh | Tugas | **Bukan** tugasnya |
| --- | --- | --- | --- |
| **Router** | `router/router.go` | Mendaftarkan URL. Semua rute fitur memakai `r.Protected(...)`. | Logika apa pun. |
| **Handler** | `handler/anggota_handler.go` | Membaca form, memanggil service, memetakan hasil ke tampilan. | Aturan bisnis, SQL. |
| **Service** | `service/anggota_service.go` | Validasi, aturan bisnis, transaksi, log audit. | HTML, mengetahui `http`. |
| **Repository** | `repository/anggota_repository.go` | Satu fungsi = satu query. | Aturan bisnis, transaksi. |
| **Model** | `model/anggota.go` | Struct data. | Perilaku. |
| **Template** | `templates/pages/anggota_*.html` | Tampilan. Memakai `layout/layout.html`. | Perhitungan. |

### Pola yang harus ditiru

1. **Service memiliki interface repository-nya sendiri** (`AnggotaRepo`, `SimpananRepo`, `PinjamanRepo` di `anggota_service.go`).
   Itu yang membuat tes bisa memakai tiruan tanpa database. Handler juga memiliki interface service-nya.
2. **Setiap service method dimulai dengan cek login**: `IdentitasDari(ctx)`, bila tidak ada mengembalikan `ErrBelumLogin`.
3. **Perubahan data = satu transaksi**: `s.db.DenganTx(ctx, func(tx database.DBTX) error { ... })`. Bila fungsi
   mengembalikan error, semuanya dibatalkan. **Di dalam fungsi itu selalu gunakan `tx`, jangan `s.db`**; query lewat
   `s.db` berjalan di luar transaksi, dan kunci `SELECT ... FOR UPDATE` tidak berlaku.
4. **Log audit ditulis di transaksi yang sama** dengan perubahannya: `s.auditSvc.NewEntri(ctx, tx, aksi, entitas, id, detail)`.
   Bila audit gagal, perubahan ikut dibatalkan (ada tesnya).
5. **Repository menerima `database.DBTX`**, sehingga fungsi yang sama bisa dipakai di dalam maupun di luar transaksi.
6. **Kesalahan dipetakan di handler** menjadi tampilan:

   | Error dari service | Respons |
   | --- | --- |
   | `*ValidasiError` | Form ditampilkan ulang dengan isian dan pesan per kolom, status **422** |
   | `ErrAnggotaTidakDitemukan` atau ID di URL tidak valid | **404** |
   | `ErrMasihAdaPinjamanAktif`, `ErrAnggotaSudahKeluar` | Halaman konfirmasi dengan alasan, status **409** |
   | lainnya | dicatat di log, pengguna melihat pesan umum, status **500** (detail tidak bocor) |

7. **Post-Redirect-Get**: setelah `POST` sukses, handler mengalihkan (`303`) ke halaman lain dengan `?berhasil=<kode>`.
   Teks pesannya dipilih di `successMessage`; teks dari URL tidak pernah ditampilkan langsung.
8. **Form wajib memuat** `<input type="hidden" name="csrf_token" value="{{.CSRF}}">`, kalau tidak, `POST` ditolak 403.
9. **Uang selalu `int64` rupiah.** Di template, tampilkan dengan `{{rupiah .Saldo.Pokok}}` (hasil: `Rp1.500.000`).
10. **Data tidak dihapus.** Anggota yang keluar diberi status `keluar`, bukan di-`DELETE`. Koreksi transaksi dengan
    membatalkan lalu memasukkan ulang (lihat `CONTRIBUTING.md`).

### Rute fitur Anggota

| Method + URL | Handler | Fungsi |
| --- | --- | --- |
| `GET /anggota` | `GetListFiltered` | Daftar, pencarian (`?q=`), filter (`?status=`), halaman (`?page=&limit=`) |
| `GET /anggota/baru` | `FormAnggota` | Form pendaftaran |
| `POST /anggota` | `SimpanAnggota` | Simpan anggota baru dan setoran simpanan pokok |
| `GET /anggota/{id}` | `DetailAnggota` | Profil, saldo simpanan, penanda pinjaman aktif |
| `GET /anggota/update/{id}` | `FormUpdate` | Form ubah nama dan telepon |
| `POST /anggota/update/{id}` | `SaveUpdateAnggota` | Simpan perubahan (log audit mencatat nilai sebelum dan sesudah) |
| `GET /anggota/keluar/{id}` | `FormAnggoaResign` | Konfirmasi: simpanan yang akan dikembalikan atau alasan penolakan |
| `POST /anggota/keluar/{id}` | `AnggotaResign` | Keluarkan anggota (lihat di bawah) |

Aturan **keluar**: ditolak bila anggota masih punya pinjaman aktif atau sudah keluar. Bila boleh, dalam
satu transaksi: seluruh saldo pokok, wajib, dan sukarela dicatat sebagai penarikan (`tipe_mutasi = 'tarik'`),
status anggota menjadi `keluar` dengan tanggal keluar hari ini (zona Asia/Jakarta), dan log audit ditulis.

### Menambah fitur baru (urutan yang disarankan)

1. Baca bagian terkait di PRD dan `docs/spec-perhitungan.md`.
2. `model/`: tambahkan struct bila perlu.
3. `repository/<fitur>.go`: fungsi SQL, menerima `database.DBTX`.
4. `service/<fitur>.go`: aturan bisnis **dan tesnya dulu** (table-driven, termasuk kasus batas). Tambahkan tiruan di `fake_test.go`.
5. `handler/<fitur>.go`: handler tipis.
6. `templates/pages/<fitur>_*.html`: halaman.
7. Daftarkan rute di `router/router.go` dengan `r.Protected(...)`, dan sambungkan service di `cmd/koperasi/main.go`
   (dua berkas bersama ini hanya diubah lewat PR kecil tersendiri; lihat `CONTRIBUTING.md`).
8. `gofmt -l .`, `go vet ./...`, `go test -race ./...`, lalu jalankan aplikasinya dan coba di browser.

---

## Alur kerja Git

- Jangan push langsung ke `develop` atau `main`. Satu tugas = satu branch = satu Pull Request.
- Branch yang dipakai sejauh ini: `feature/<nama-fitur>` (misalnya `feature/anggota`), dibuka PR-nya ke `develop`.
- Ambil kode terbaru sebelum mulai: `git checkout develop && git pull`, lalu `git checkout -b feature/<nama>`.
- Pesan commit mengikuti [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/), misalnya
  `feat: detail & update anggota`, `fix: ...`, `docs: ...`, `test: ...`.
- PR butuh minimal satu persetujuan dan CI hijau sebelum di-merge. Perubahan di `migrations/`, `internal/helper/` (perhitungan uang),
  dan berkas bersama membutuhkan persetujuan ketua tim.
- Perubahan skema = berkas migrasi baru bernomor urut (`000004_xxx.up.sql` dan `.down.sql`). Minta nomor ke ketua tim.
  **Jangan mengubah migrasi yang sudah di-merge.**

---

## Masalah yang sering muncul

| Gejala | Penyebab dan solusi |
| --- | --- |
| `database: DSN harus memuat parseTime=true` | `KOPERASI_DSN` di `.env` belum benar atau `.env` tidak terbaca. Jalankan dari akar repo. |
| `config: ...` atau `Error 1045 Access denied` | Salah nama/kata sandi pengguna MySQL di DSN, atau `GRANT` belum dijalankan. |
| `migrate: command not found` | Belum terpasang, atau `$(go env GOPATH)/bin` belum ada di `PATH`. |
| `Dirty database version N` | Migrasi N gagal di tengah. Perbaiki penyebabnya, lalu `migrate ... force N-1` dan jalankan `up` lagi (atau buat ulang database lokal). |
| `go: go.mod requires go >= 1.27.0` | Versi Go terlalu lama. Perbarui Go. |
| Login `pengurus1` selalu gagal | Wajar: kata sandi data contoh masih placeholder. Pakai `admin`. |
| Halaman `POST` mengembalikan 403 | Form tidak memuat `csrf_token`, atau sesi sudah berakhir. Login ulang. |
| `address already in use` | Port 8080 terpakai. Ubah `KOPERASI_ALAMAT` di `.env`. |
