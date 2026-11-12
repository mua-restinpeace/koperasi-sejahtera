-- =====================================================================
-- Dummy / seed data: Koperasi Karyawan Sejahtera
-- Target: MySQL 8.0.16+
-- Jalankan SETELAH schema.sql berhasil dieksekusi.
--
-- Catatan:
-- 1) Password hash sengaja berupa placeholder.
-- 2) Token sesi di bawah adalah data dummy, bukan token login sungguhan.
-- 3) ID dibuat eksplisit agar relasi antartabel mudah dibaca.
-- 4) Script ini ditujukan untuk database development/test, bukan production.
-- =====================================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- Kosongkan tabel dalam urutan child -> parent agar seed dapat diulang.
TRUNCATE TABLE alokasi_pembayaran;
TRUNCATE TABLE pembayaran_pinjaman;
TRUNCATE TABLE angsuran;
TRUNCATE TABLE pinjaman;
TRUNCATE TABLE transaksi_simpanan;
TRUNCATE TABLE sesi;
TRUNCATE TABLE log_audit;
TRUNCATE TABLE anggota;
TRUNCATE TABLE pengguna;

SET FOREIGN_KEY_CHECKS = 1;

-- ---------------------------------------------------------------------
-- PENGGUNA
-- Ganti REPLACE_WITH_BCRYPT_HASH dengan hash password yang valid.
-- Password untuk admin=adminpassword99
-- ---------------------------------------------------------------------
INSERT INTO pengguna
(id, username, password_hash, nama_lengkap, aktif, dibuat_pada)
VALUES
(1, 'admin', 'pbkdf2-sha256$600000$iLvMY18FqfLR5mBMnXPo3A$uz/SeTcUgGS5ua+/on0JS5VTMIhCdlEm/BPWIyZ5Uho', 'Administrator Koperasi', 1, '2025-01-02 01:00:00'),
(2, 'pengurus1', 'REPLACE_WITH_BCRYPT_HASH', 'Siti Rahmawati', 1, '2025-01-02 01:05:00'),
(3, 'pengurus.nonaktif', 'REPLACE_WITH_BCRYPT_HASH', 'Budi Santoso', 0, '2025-01-03 02:00:00');

-- ---------------------------------------------------------------------
-- ANGGOTA
-- ---------------------------------------------------------------------
INSERT INTO anggota
(id, nomor_anggota, nama_lengkap, no_telepon, tanggal_bergabung, status, tanggal_keluar, dibuat_pada)
VALUES
(1, 'KKS-0001', 'Andi Pratama', '081234560001', '2023-01-10', 'aktif', NULL, '2025-01-02 02:00:00'),
(2, 'KKS-0002', 'Dewi Anggraini', '081234560002', '2023-03-15', 'aktif', NULL, '2025-01-02 02:05:00'),
(3, 'KKS-0003', 'Rizky Firmansyah', '081234560003', '2022-06-01', 'keluar', '2025-12-31', '2025-01-02 02:10:00'),
(4, 'KKS-0004', 'Maya Putri', '081234560004', '2024-02-20', 'aktif', NULL, '2025-01-02 02:15:00'),
(5, 'KKS-0005', 'Agus Setiawan', NULL, '2021-08-05', 'keluar', '2024-11-30', '2025-01-02 02:20:00'),
(6, 'KKS-0006', 'Nadia Safitri', '081234560006', '2025-01-05', 'aktif', NULL, '2025-01-05 01:00:00');

-- ---------------------------------------------------------------------
-- TRANSAKSI SIMPANAN
-- Contoh mencakup pokok, wajib, sukarela, penarikan sukarela, dan batal.
-- Untuk simpanan wajib, periode harus tanggal 1 setiap bulan.
-- ---------------------------------------------------------------------
INSERT INTO transaksi_simpanan
(id, id_anggota, kategori, tipe_mutasi, jumlah, periode, tanggal_transaksi,
 keterangan, dibuat_oleh, dibuat_pada, dibatalkan_pada, dibatalkan_oleh, alasan_pembatalan)
VALUES
(1, 1, 'pokok', 'setor', 100000, NULL, '2023-01-10', 'Setoran simpanan pokok awal', 1, '2025-01-03 01:00:00', NULL, NULL, NULL),
(2, 2, 'pokok', 'setor', 100000, NULL, '2023-03-15', 'Setoran simpanan pokok awal', 1, '2025-01-03 01:05:00', NULL, NULL, NULL),
(3, 3, 'pokok', 'setor', 100000, NULL, '2022-06-01', 'Setoran simpanan pokok awal', 1, '2025-01-03 01:10:00', NULL, NULL, NULL),
(4, 4, 'pokok', 'setor', 100000, NULL, '2024-02-20', 'Setoran simpanan pokok awal', 1, '2025-01-03 01:15:00', NULL, NULL, NULL),
(5, 5, 'pokok', 'setor', 100000, NULL, '2021-08-05', 'Setoran simpanan pokok awal', 1, '2025-01-03 01:20:00', NULL, NULL, NULL),
(6, 6, 'pokok', 'setor', 100000, NULL, '2025-01-05', 'Setoran simpanan pokok awal', 1, '2025-01-05 01:10:00', NULL, NULL, NULL),

(7, 1, 'wajib', 'setor', 50000, '2026-01-01', '2026-01-05', 'Simpanan wajib Januari 2026', 2, '2026-01-05 02:00:00', NULL, NULL, NULL),
(8, 1, 'wajib', 'setor', 50000, '2026-02-01', '2026-02-05', 'Simpanan wajib Februari 2026', 2, '2026-02-05 02:00:00', NULL, NULL, NULL),
(9, 1, 'wajib', 'setor', 50000, '2026-03-01', '2026-03-05', 'Simpanan wajib Maret 2026', 2, '2026-03-05 02:00:00', NULL, NULL, NULL),
(10, 2, 'wajib', 'setor', 50000, '2026-01-01', '2026-01-07', 'Simpanan wajib Januari 2026', 2, '2026-01-07 02:00:00', NULL, NULL, NULL),
(11, 2, 'wajib', 'setor', 50000, '2026-02-01', '2026-02-07', 'Simpanan wajib Februari 2026', 2, '2026-02-07 02:00:00', NULL, NULL, NULL),
(12, 4, 'wajib', 'setor', 50000, '2026-01-01', '2026-01-10', 'Simpanan wajib Januari 2026', 2, '2026-01-10 02:00:00', NULL, NULL, NULL),
(13, 6, 'wajib', 'setor', 50000, '2026-02-01', '2026-02-05', 'Simpanan wajib Februari 2026', 2, '2026-02-05 02:10:00', NULL, NULL, NULL),

(14, 1, 'sukarela', 'setor', 250000, NULL, '2026-01-12', 'Setoran sukarela', 2, '2026-01-12 03:00:00', NULL, NULL, NULL),
(15, 1, 'sukarela', 'tarik', 50000, NULL, '2026-02-20', 'Penarikan simpanan sukarela', 2, '2026-02-20 03:00:00', NULL, NULL, NULL),
(16, 2, 'sukarela', 'setor', 500000, NULL, '2026-02-12', 'Tambahan simpanan sukarela', 2, '2026-02-12 03:00:00', NULL, NULL, NULL),
(17, 4, 'sukarela', 'setor', 150000, NULL, '2026-03-01', 'Setoran sukarela', 2, '2026-03-01 03:00:00', NULL, NULL, NULL),
(18, 6, 'sukarela', 'setor', 100000, NULL, '2026-02-10', 'Setoran sukarela pertama', 2, '2026-02-10 03:00:00', NULL, NULL, NULL),
(19, 1, 'sukarela', 'setor', 75000, NULL, '2026-03-10', 'Transaksi salah input - dibatalkan', 2, '2026-03-10 03:00:00', '2026-03-10 04:00:00', 1, 'Nominal transaksi tidak sesuai bukti setoran');

-- ---------------------------------------------------------------------
-- PINJAMAN
-- Pinjaman 1 aktif, pinjaman 2 lunas, pinjaman 3 dibatalkan.
-- Jasa flat 1% per bulan (100 bps).
-- ---------------------------------------------------------------------
INSERT INTO pinjaman
(id, id_anggota, jumlah_pinjaman, tenor_bulan, jasa_per_bulan_bps, denda_per_hari,
 tanggal_pencairan, status, tanggal_lunas, dibuat_oleh, dibuat_pada)
VALUES
(1, 1, 12000000, 12, 100, 2000, '2026-01-15', 'aktif', NULL, 1, '2026-01-15 02:00:00'),
(2, 2, 6000000, 6, 100, 2000, '2025-01-10', 'lunas', '2025-07-10', 1, '2025-01-10 02:00:00'),
(3, 3, 3000000, 6, 100, 2000, '2025-08-01', 'dibatalkan', NULL, 1, '2025-08-01 02:00:00');

-- ---------------------------------------------------------------------
-- ANGSURAN
-- Pinjaman 1: pokok 1.000.000 + jasa 120.000 x 12 bulan.
-- Pinjaman 2: pokok 1.000.000 + jasa 60.000 x 6 bulan.
-- Angsuran terakhir menyerap sisa pembulatan (tidak ada sisa pada contoh ini).
-- ---------------------------------------------------------------------
INSERT INTO angsuran
(id, id_pinjaman, urutan, tanggal_jatuh_tempo, pokok_tagihan, jasa_tagihan)
VALUES
(1, 1, 1, '2026-02-15', 1000000, 120000),
(2, 1, 2, '2026-03-15', 1000000, 120000),
(3, 1, 3, '2026-04-15', 1000000, 120000),
(4, 1, 4, '2026-05-15', 1000000, 120000),
(5, 1, 5, '2026-06-15', 1000000, 120000),
(6, 1, 6, '2026-07-15', 1000000, 120000),
(7, 1, 7, '2026-08-15', 1000000, 120000),
(8, 1, 8, '2026-09-15', 1000000, 120000),
(9, 1, 9, '2026-10-15', 1000000, 120000),
(10, 1, 10, '2026-11-15', 1000000, 120000),
(11, 1, 11, '2026-12-15', 1000000, 120000),
(12, 1, 12, '2027-01-15', 1000000, 120000),

(13, 2, 1, '2025-02-10', 1000000, 60000),
(14, 2, 2, '2025-03-10', 1000000, 60000),
(15, 2, 3, '2025-04-10', 1000000, 60000),
(16, 2, 4, '2025-05-10', 1000000, 60000),
(17, 2, 5, '2025-06-10', 1000000, 60000),
(18, 2, 6, '2025-07-10', 1000000, 60000);

-- ---------------------------------------------------------------------
-- PEMBAYARAN PINJAMAN
-- Pinjaman 1: satu angsuran dibayar penuh.
-- Pinjaman 2: pelunasan seluruh 6 angsuran sekaligus + biaya administrasi.
-- ---------------------------------------------------------------------
INSERT INTO pembayaran_pinjaman
(id, id_pinjaman, jenis, tanggal_bayar, jumlah_diterima, biaya_administrasi,
 keterangan, dibuat_oleh, dibuat_pada, dibatalkan_pada, dibatalkan_oleh, alasan_pembatalan)
VALUES
(1, 1, 'angsuran', '2026-02-15', 1120000, 0, 'Pembayaran angsuran ke-1', 2, '2026-02-15 03:00:00', NULL, NULL, NULL),
(2, 2, 'pelunasan', '2025-07-10', 6385000, 25000, 'Pelunasan seluruh sisa pinjaman', 1, '2025-07-10 03:00:00', NULL, NULL, NULL);

-- ---------------------------------------------------------------------
-- ALOKASI PEMBAYARAN
-- Total alokasi pembayaran #1 = 1.120.000.
-- Total alokasi pembayaran #2 = 6.360.000; admin 25.000 disimpan di pembayaran.
-- ---------------------------------------------------------------------
INSERT INTO alokasi_pembayaran
(id, id_pembayaran, id_angsuran, denda_dibayar, jasa_dibayar, pokok_dibayar)
VALUES
(1, 1, 1, 0, 120000, 1000000),
(2, 2, 13, 0, 60000, 1000000),
(3, 2, 14, 0, 60000, 1000000),
(4, 2, 15, 0, 60000, 1000000),
(5, 2, 16, 0, 60000, 1000000),
(6, 2, 17, 0, 60000, 1000000),
(7, 2, 18, 0, 60000, 1000000);

-- ---------------------------------------------------------------------
-- LOG AUDIT
-- ---------------------------------------------------------------------
INSERT INTO log_audit
(id, id_pengguna, aksi, entitas, id_entitas, detail, dibuat_pada)
VALUES
(1, 1, 'CREATE', 'anggota', 1, JSON_OBJECT('nomor_anggota', 'KKS-0001', 'nama', 'Andi Pratama'), '2025-01-02 02:00:01'),
(2, 1, 'CREATE', 'pinjaman', 2, JSON_OBJECT('jumlah_pinjaman', 6000000, 'tenor_bulan', 6), '2025-01-10 02:00:01'),
(3, 2, 'CREATE', 'pembayaran_pinjaman', 1, JSON_OBJECT('jumlah_diterima', 1120000, 'jenis', 'angsuran'), '2026-02-15 03:00:01'),
(4, 1, 'CANCEL', 'transaksi_simpanan', 19, JSON_OBJECT('alasan', 'Nominal transaksi tidak sesuai bukti setoran'), '2026-03-10 04:00:01'),
(5, 1, 'SETTLE', 'pinjaman', 2, JSON_OBJECT('tanggal_lunas', '2025-07-10'), '2025-07-10 03:00:01');

-- ---------------------------------------------------------------------
-- SESI
-- token_hash dan csrf_token hanyalah nilai dummy untuk pengujian.
-- ---------------------------------------------------------------------
INSERT INTO sesi
(id, id_pengguna, token_hash, csrf_token, dibuat_pada, kedaluwarsa_pada)
VALUES
(1, 1, SHA2('dummy-session-token-admin', 256), 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMN', '2026-11-12 08:00:00', '2026-11-12 16:00:00'),
(2, 2, SHA2('dummy-session-token-bendahara', 256), '1234567890123456789012345678901234567890123', '2026-11-12 08:30:00', '2026-11-12 16:30:00');

-- =====================================================================
-- SELESAI
-- =====================================================================
