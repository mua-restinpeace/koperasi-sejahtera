## Ringkasan

<!-- Apa yang berubah dan kenapa, dalam 1-3 kalimat. -->

Menutup issue: #

## Cara mencoba

<!-- Langkah singkat supaya pemeriksa bisa menjalankannya. -->

## Daftar periksa

- [ ] `make fmt` dan `make test` hijau di komputer saya
- [ ] Aturan bisnis ada di `service`, bukan di handler
- [ ] Uang memakai `int64` (tidak ada `float`)
- [ ] Aturan baru punya tes tabel termasuk kasus batas
- [ ] Perubahan data menulis log audit
- [ ] Tidak ada `DELETE`/`UPDATE` atas transaksi (pembatalan, bukan penghapusan)
- [ ] Migrasi baru memakai nomor dari ketua tim dan tidak mengubah migrasi lama
- [ ] Tidak mengubah berkas milik fitur lain tanpa berdiskusi