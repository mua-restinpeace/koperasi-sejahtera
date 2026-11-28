-- =====================================================================
-- Seed "Contoh C" (spec-perhitungan.md bagian 6) untuk menguji Laporan.
-- Aman dijalankan berulang: hanya menyentuh baris ber-ID 900-an dan TIDAK
-- menghapus data lain. Jalankan SETELAH migrasi dan 000003_dummy_data.
--
--   mysql -u koperasi -p koperasi < seed/contoh_c.sql
--
-- Pinjaman Rp6.000.000, tenor 6, cair 2026-11-05, jasa 1%, denda Rp2.000/hari.
-- Seluruh pembayaran jatuh di 2027, jadi pilih bulan 2027-01 .. 2027-02 di L-3
-- dan tanggal 2027-02-11 di L-2.
--
-- Angka yang HARUS muncul:
--  L-3 (2027-01 s/d 2027-02):
--      Januari 2027   jasa 120.000  denda 82.000  admin 0        total 202.000
--      Februari 2027  jasa 120.000  denda 76.000  admin 80.000   total 276.000
--      Total          jasa 240.000  denda 158.000 admin 80.000   total 478.000
--    (pembayaran #9904 yang DIBATALKAN tidak boleh ikut: kalau ikut, jasa Jan = 180.000)
--  L-2 pada 2027-02-11 (anggota KKS-0900):
--      angsuran 2: terlambat 32 hari, sisa jasa 0,      sisa pokok 702.000,   denda 54.000
--      angsuran 3: terlambat 1 hari,  sisa jasa 60.000, sisa pokok 1.000.000, denda 2.000
--  L-2 pada tanggal hari ini (2026-11): kosong, belum ada angsuran jatuh tempo.
-- =====================================================================

SET NAMES utf8mb4;

DELETE FROM alokasi_pembayaran WHERE id_pembayaran BETWEEN 9901 AND 9904;
DELETE FROM pembayaran_pinjaman WHERE id BETWEEN 9901 AND 9904;
DELETE FROM angsuran WHERE id_pinjaman = 900;
DELETE FROM pinjaman WHERE id = 900;
DELETE FROM anggota WHERE id = 900;

INSERT INTO anggota (id, nomor_anggota, nama_lengkap, no_telepon, tanggal_bergabung, status)
VALUES (900, 'KKS-0900', 'Contoh C (Uji Laporan)', '0800-0000-0900', '2026-01-05', 'aktif');

INSERT INTO pinjaman (id, id_anggota, jumlah_pinjaman, tenor_bulan, jasa_per_bulan_bps, denda_per_hari,
                      tanggal_pencairan, status, tanggal_lunas, dibuat_oleh)
VALUES (900, 900, 6000000, 6, 100, 2000, '2026-11-05', 'lunas', '2027-02-20', 1);

INSERT INTO angsuran (id, id_pinjaman, urutan, tanggal_jatuh_tempo, pokok_tagihan, jasa_tagihan) VALUES
(9001, 900, 1, '2026-12-10', 1000000, 60000),
(9002, 900, 2, '2027-01-10', 1000000, 60000),
(9003, 900, 3, '2027-02-10', 1000000, 60000),
(9004, 900, 4, '2027-03-10', 1000000, 60000),
(9005, 900, 5, '2027-04-10', 1000000, 60000),
(9006, 900, 6, '2027-05-10', 1000000, 60000);

-- 9901: Rp1.500.000 pada 2027-01-15 | 9902: Rp800.000 pada 2027-02-12
-- 9903: pelunasan dipercepat Rp4.178.000 (termasuk admin 80.000) pada 2027-02-20
-- 9904: pembayaran yang DIBATALKAN (harus diabaikan semua laporan)
INSERT INTO pembayaran_pinjaman (id, id_pinjaman, jenis, tanggal_bayar, jumlah_diterima, biaya_administrasi,
                                 keterangan, dibuat_oleh, dibatalkan_pada, dibatalkan_oleh, alasan_pembatalan) VALUES
(9901, 900, 'angsuran',  '2027-01-15', 1500000,       0, 'Contoh C pembayaran 1', 1, NULL, NULL, NULL),
(9902, 900, 'angsuran',  '2027-02-12',  800000,       0, 'Contoh C pembayaran 2', 1, NULL, NULL, NULL),
(9903, 900, 'pelunasan', '2027-02-20', 4178000,   80000, 'Contoh C pelunasan',    1, NULL, NULL, NULL),
(9904, 900, 'angsuran',  '2027-01-20',  100000,       0, 'Salah input (uji pembatalan)', 1, '2027-01-21 09:00:00', 1, 'salah input');

INSERT INTO alokasi_pembayaran (id_pembayaran, id_angsuran, denda_dibayar, jasa_dibayar, pokok_dibayar) VALUES
(9901, 9001, 72000, 60000, 1000000),
(9901, 9002, 10000, 60000,  298000),
(9902, 9002, 56000,     0,  702000),
(9902, 9003,  4000, 38000,       0),
(9903, 9003, 16000, 22000, 1000000),
(9903, 9004,     0, 60000, 1000000),
(9903, 9005,     0,     0, 1000000),
(9903, 9006,     0,     0, 1000000),
(9904, 9003,     0, 60000,   40000);
