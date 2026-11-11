# Setup lokal

Panduan menjalankan aplikasi di laptop sendiri. Butuh: Go (sesuai `go.mod`), MySQL 8.0.16 ke atas, dan [golang-migrate CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) (dengan driver mysql).

## 1. Unduh dependensi

```sh
go get github.com/go-sql-driver/mysql
go mod tidy
```

Jalankan sekali setelah menarik cabang ini, lalu commit perubahan `go.mod` dan `go.sum`.

## 2. Siapkan database

Satu database untuk pengembangan, satu lagi khusus tes (nama wajib berakhiran `_test`):

```sql
CREATE DATABASE koperasi      CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE koperasi_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'koperasi'@'%' IDENTIFIED BY 'koperasi';
GRANT ALL ON koperasi.*      TO 'koperasi'@'%';
GRANT ALL ON koperasi_test.* TO 'koperasi'@'%';
```

## 3. Konfigurasi

```sh
cp .env.example .env
```

Isi `KOPERASI_DSN`. DSN **wajib** memuat `parseTime=true` (aplikasi menolak start tanpanya). Disarankan juga `loc=UTC` dan `time_zone='+00:00'` agar semua timestamp konsisten UTC:

```
KOPERASI_DSN=koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi?parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27
```

Saat mengakses lewat HTTP biasa (lokal), biarkan `KOPERASI_COOKIE_SECURE` kosong atau `false`. Di server dengan HTTPS, set `true`.

## 4. Migrasi

```sh
make migrasi-naik      # menerapkan semua migrasi
make migrasi-turun     # membatalkan satu migrasi terakhir
```

URL database migrasi dapat diganti: `make migrasi-naik MIGRATE_URL='mysql://user:pass@tcp(host:3306)/db?multiStatements=true'`.
Migrasi baru: buat sepasang berkas `migrations/00000N_nama.up.sql` dan `.down.sql`. Jangan mengubah migrasi yang sudah di-merge ke `main`.

## 5. Buat akun pengurus

```sh
make buat-pengguna ARGS='-username ani -nama "Ani Lestari"'
```

Kata sandi diketik setelah diminta (minimal 10 karakter; terlihat di layar, dibaca dari stdin supaya tidak masuk riwayat shell).

## 6. Jalankan

```sh
make run
```

Buka http://localhost:8080 lalu login.

## Tes

```sh
make test              # tes unit, tanpa database
KOPERASI_TEST_DSN='koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi_test?parseTime=true&loc=UTC' make migrasi-naik MIGRATE_URL='mysql://koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi_test?multiStatements=true'
KOPERASI_TEST_DSN='koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi_test?parseTime=true&loc=UTC' make test-integrasi
```

Tes integrasi **mengosongkan semua tabel** di database tes, dan menolak berjalan bila nama database tidak berakhiran `_test`.

## Catatan keamanan yang perlu diketahui

- Form login belum memiliki token CSRF dan belum ada pembatasan percobaan login. Untuk aplikasi internal dengan 2 pengurus ini dapat diterima; tambahkan pembatasan bila diakses dari internet.
- Kegagalan login dicatat di log aplikasi (`slog`), bukan di tabel `log_audit`, karena tabel itu mensyaratkan pengguna yang dikenal.
