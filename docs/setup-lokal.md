# Setup lokal (ringkas)

Langkah lengkap, dengan penjelasan, ada di [`docs/README.md`](README.md#setup-di-laptop-sendiri). Halaman ini hanya
merangkum perintahnya dan menjelaskan **tes integrasi**. Tidak perlu `make`; [`Makefile`](../Makefile) hanya merangkum
perintah yang sama.

Butuh: Go (sesuai `go.mod`), MySQL 8.0.16 ke atas, dan [golang-migrate CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) dengan driver mysql.

## 1. Database

Satu database untuk pengembangan, satu lagi khusus tes (nama wajib berakhiran `_test`):

```sql
CREATE DATABASE koperasi      CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE koperasi_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'koperasi'@'%' IDENTIFIED BY 'koperasi';
GRANT ALL ON koperasi.*      TO 'koperasi'@'%';
GRANT ALL ON koperasi_test.* TO 'koperasi'@'%';
```

## 2. Konfigurasi

```sh
cp .env.example .env
```

Isi `KOPERASI_DSN`. DSN **wajib** memuat `parseTime=true` (aplikasi menolak start tanpanya); disarankan juga `loc=UTC`:

```
KOPERASI_DSN=koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi?parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27
```

Lewat HTTP biasa (lokal) biarkan `KOPERASI_COOKIE_SECURE` kosong; di server HTTPS set `true`.

## 3. Migrasi dan data contoh

```sh
migrate -path migrations -database "mysql://koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi?multiStatements=true" up
mysql -u koperasi -p koperasi -e "source migrations/000003_dummy_data_koperasi.sql"   # opsional
```

- Untuk membatalkan satu migrasi terakhir, ganti `up` dengan `down 1`.
- Migrasi baru: sepasang berkas `migrations/00000N_nama.up.sql` dan `.down.sql`. Jangan mengubah migrasi yang sudah di-merge ke `main`.
- Berkas data contoh diawali `TRUNCATE` pada **semua tabel**; jangan dijalankan pada database yang datanya ingin disimpan.

## 4. Akun pengurus

Belum ada perintah untuk membuat akun. Data contoh membawa satu akun yang bisa login:

| Username | Kata sandi |
| --- | --- |
| `admin` | `adminpassword99` |

Hanya untuk pengembangan; jangan dibiarkan ada di server sungguhan.

## 5. Jalankan

```sh
go run ./cmd/koperasi
```

Buka <http://localhost:8080> lalu login.

## Tes

```sh
go vet ./...
go test -race ./...            # tes unit, tanpa database
```

### Tes integrasi (MySQL sungguhan)

Tes integrasi (tag `integration`, mis. `internal/service/simpanan_integrasi_test.go`) membuktikan hal yang tidak dapat
dibuktikan oleh tiruan: unique key per periode simpanan wajib, `CAST` pada saldo, dan penguncian baris saat
penarikan/pembatalan bersamaan. Tes dilewati bila `KOPERASI_TEST_DSN` kosong.

```sh
KOPERASI_TEST_DSN='koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi_test?parseTime=true&loc=UTC' \
    go test -tags integration -count=1 -race ./internal/...
```

> **Awas:** tes **menghapus semua tabel** di database itu lalu membuatnya ulang dari `migrations/*.up.sql`. Tes menolak
> berjalan bila nama database tidak berakhiran `_test`. Tidak perlu menjalankan `migrate` pada database tes.

`make test-integrasi` menjalankan perintah yang sama (isi `KOPERASI_TEST_DSN` lebih dulu).

## Catatan keamanan yang perlu diketahui

- Belum ada pembatasan percobaan login. Untuk aplikasi internal dengan beberapa pengurus ini dapat diterima; tambahkan pembatasan bila diakses dari internet.
- Kegagalan login dicatat di log aplikasi (`slog`), bukan di tabel `log_audit`, karena tabel itu mensyaratkan pengguna yang dikenal.
