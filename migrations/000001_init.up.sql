-- =====================================================================
-- Skema Aplikasi Koperasi Karyawan Sejahtera
-- Target: MySQL 8.0.16+ (CHECK constraint baru ditegakkan mulai versi ini)
-- Semua nominal dalam rupiah utuh (BIGINT), tanpa desimal.
-- Semua tanggal bisnis memakai DATE (zona waktu Asia/Jakarta);
-- kolom dibuat_pada dll. memakai DATETIME dalam UTC.
-- =====================================================================

SET NAMES utf8mb4;

-- ---------------------------------------------------------------------
-- pengguna: akun pengurus yang login ke aplikasi
-- ---------------------------------------------------------------------
CREATE TABLE pengguna (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username      VARCHAR(50)     NOT NULL,
    password_hash VARCHAR(255)    NOT NULL,
    nama_lengkap  VARCHAR(100)    NOT NULL,
    aktif         TINYINT(1)      NOT NULL DEFAULT 1,
    dibuat_pada   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_pengguna_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------
-- anggota
-- ---------------------------------------------------------------------
CREATE TABLE anggota (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    nomor_anggota     VARCHAR(20)     NOT NULL,
    nama_lengkap      VARCHAR(100)    NOT NULL,
    no_telepon        VARCHAR(20)     NULL,
    tanggal_bergabung DATE            NOT NULL,
    status            ENUM('aktif','keluar') NOT NULL DEFAULT 'aktif',
    tanggal_keluar    DATE            NULL,
    dibuat_pada       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_anggota_nomor (nomor_anggota),
    CONSTRAINT ck_anggota_keluar CHECK (
        (status = 'aktif' AND tanggal_keluar IS NULL)
        OR (status = 'keluar' AND tanggal_keluar IS NOT NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------
-- transaksi_simpanan: buku besar simpanan (pokok, wajib, sukarela).
-- Tidak pernah di-UPDATE nominalnya; koreksi = batalkan + input ulang.
-- Penarikan pokok/wajib hanya boleh saat anggota keluar (dijaga di aplikasi).
-- Saldo = SUM(setor) - SUM(tarik) untuk baris yang belum dibatalkan.
-- ---------------------------------------------------------------------
CREATE TABLE transaksi_simpanan (
    id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    id_anggota         BIGINT UNSIGNED NOT NULL,
    kategori           ENUM('pokok','wajib','sukarela') NOT NULL,
    tipe_mutasi        ENUM('setor','tarik') NOT NULL,
    jumlah             BIGINT UNSIGNED NOT NULL,
    periode            DATE            NULL COMMENT 'khusus wajib setor: tanggal 1 bulan yang dibayar',
    tanggal_transaksi  DATE            NOT NULL,
    keterangan         VARCHAR(255)    NULL,
    dibuat_oleh        BIGINT UNSIGNED NOT NULL,
    dibuat_pada        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    dibatalkan_pada    DATETIME        NULL,
    dibatalkan_oleh    BIGINT UNSIGNED NULL,
    alasan_pembatalan  VARCHAR(255)    NULL,

    -- MySQL tidak punya partial unique index; dipakai kolom generated
    -- yang bernilai NULL (dan NULL tidak dianggap duplikat) kecuali baris yang dibatasi.
    kunci_pokok   BIGINT UNSIGNED GENERATED ALWAYS AS (
        IF(kategori = 'pokok' AND tipe_mutasi = 'setor' AND dibatalkan_pada IS NULL, id_anggota, NULL)
    ) STORED,
    kunci_periode DATE GENERATED ALWAYS AS (
        IF(kategori = 'wajib' AND tipe_mutasi = 'setor' AND dibatalkan_pada IS NULL, periode, NULL)
    ) STORED,

    PRIMARY KEY (id),
    UNIQUE KEY uq_simpanan_pokok_sekali (kunci_pokok),
    UNIQUE KEY uq_simpanan_wajib_per_periode (id_anggota, kunci_periode),
    KEY ix_simpanan_anggota (id_anggota, kategori),
    KEY ix_simpanan_periode (periode),
    CONSTRAINT fk_simpanan_anggota   FOREIGN KEY (id_anggota)      REFERENCES anggota (id),
    CONSTRAINT fk_simpanan_pembuat   FOREIGN KEY (dibuat_oleh)     REFERENCES pengguna (id),
    CONSTRAINT fk_simpanan_pembatal  FOREIGN KEY (dibatalkan_oleh) REFERENCES pengguna (id),
    CONSTRAINT ck_simpanan_jumlah    CHECK (jumlah > 0),
    CONSTRAINT ck_simpanan_periode   CHECK (
        (kategori = 'wajib' AND tipe_mutasi = 'setor' AND periode IS NOT NULL AND DAY(periode) = 1)
        OR NOT (kategori = 'wajib' AND tipe_mutasi = 'setor')
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------
-- pinjaman: pengajuan dan pencairan jadi satu langkah (tanpa persetujuan)
-- ---------------------------------------------------------------------
CREATE TABLE pinjaman (
    id                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    id_anggota           BIGINT UNSIGNED NOT NULL,
    jumlah_pinjaman      BIGINT UNSIGNED NOT NULL,
    tenor_bulan          TINYINT UNSIGNED NOT NULL,
    jasa_per_bulan_bps   SMALLINT UNSIGNED NOT NULL DEFAULT 100 COMMENT '100 bps = 1% per bulan, flat',
    denda_per_hari       INT UNSIGNED    NOT NULL DEFAULT 2000,
    tanggal_pencairan    DATE            NOT NULL,
    status               ENUM('aktif','lunas','dibatalkan') NOT NULL DEFAULT 'aktif',
    tanggal_lunas        DATE            NULL,
    dibuat_oleh          BIGINT UNSIGNED NOT NULL,
    dibuat_pada          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- menjamin satu anggota hanya punya satu pinjaman aktif
    kunci_aktif BIGINT UNSIGNED GENERATED ALWAYS AS (
        IF(status = 'aktif', id_anggota, NULL)
    ) STORED,

    PRIMARY KEY (id),
    UNIQUE KEY uq_pinjaman_satu_aktif (kunci_aktif),
    KEY ix_pinjaman_anggota (id_anggota),
    CONSTRAINT fk_pinjaman_anggota FOREIGN KEY (id_anggota)  REFERENCES anggota (id),
    CONSTRAINT fk_pinjaman_pembuat FOREIGN KEY (dibuat_oleh) REFERENCES pengguna (id),
    CONSTRAINT ck_pinjaman_jumlah  CHECK (jumlah_pinjaman > 0),
    CONSTRAINT ck_pinjaman_tenor   CHECK (tenor_bulan BETWEEN 6 AND 24),
    CONSTRAINT ck_pinjaman_lunas   CHECK (
        (status = 'lunas' AND tanggal_lunas IS NOT NULL) OR status <> 'lunas'
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------
-- angsuran: jadwal angsuran, dibuat sekali saat pencairan.
-- Angsuran terakhir menyerap sisa pembulatan.
-- ---------------------------------------------------------------------
CREATE TABLE angsuran (
    id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    id_pinjaman         BIGINT UNSIGNED NOT NULL,
    urutan              TINYINT UNSIGNED NOT NULL,
    tanggal_jatuh_tempo DATE            NOT NULL,
    pokok_tagihan       BIGINT UNSIGNED NOT NULL,
    jasa_tagihan        BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_angsuran_urutan (id_pinjaman, urutan),
    KEY ix_angsuran_jatuh_tempo (tanggal_jatuh_tempo),
    CONSTRAINT fk_angsuran_pinjaman FOREIGN KEY (id_pinjaman) REFERENCES pinjaman (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------
-- pembayaran_pinjaman: uang yang diterima pengurus.
-- jumlah_diterima = SUM(alokasi denda+jasa+pokok) + biaya_administrasi
-- Koreksi = batalkan + input ulang; alokasi dihitung ulang berurutan
-- berdasarkan tanggal_bayar untuk semua pembayaran yang belum dibatalkan.
-- ---------------------------------------------------------------------
CREATE TABLE pembayaran_pinjaman (
    id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    id_pinjaman         BIGINT UNSIGNED NOT NULL,
    jenis               ENUM('angsuran','pelunasan') NOT NULL,
    tanggal_bayar       DATE            NOT NULL COMMENT 'tanggal bisnis, boleh mundur',
    jumlah_diterima     BIGINT UNSIGNED NOT NULL,
    biaya_administrasi  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'hanya pelunasan; pendapatan koperasi',
    keterangan          VARCHAR(255)    NULL,
    dibuat_oleh         BIGINT UNSIGNED NOT NULL,
    dibuat_pada         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    dibatalkan_pada     DATETIME        NULL,
    dibatalkan_oleh     BIGINT UNSIGNED NULL,
    alasan_pembatalan   VARCHAR(255)    NULL,
    PRIMARY KEY (id),
    KEY ix_pembayaran_pinjaman (id_pinjaman, tanggal_bayar),
    KEY ix_pembayaran_tanggal (tanggal_bayar),
    CONSTRAINT fk_pembayaran_pinjaman FOREIGN KEY (id_pinjaman)    REFERENCES pinjaman (id),
    CONSTRAINT fk_pembayaran_pembuat  FOREIGN KEY (dibuat_oleh)    REFERENCES pengguna (id),
    CONSTRAINT fk_pembayaran_pembatal FOREIGN KEY (dibatalkan_oleh) REFERENCES pengguna (id),
    CONSTRAINT ck_pembayaran_jumlah   CHECK (jumlah_diterima > 0),
    CONSTRAINT ck_pembayaran_admin    CHECK (jenis = 'pelunasan' OR biaya_administrasi = 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------
-- alokasi_pembayaran: pembagian tiap pembayaran ke denda, jasa, pokok
-- per angsuran. Denda terhitung (tidak disimpan sebagai saldo);
-- hanya denda yang sudah dibayar yang dicatat di sini.
-- ---------------------------------------------------------------------
CREATE TABLE alokasi_pembayaran (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    id_pembayaran  BIGINT UNSIGNED NOT NULL,
    id_angsuran    BIGINT UNSIGNED NOT NULL,
    denda_dibayar  BIGINT UNSIGNED NOT NULL DEFAULT 0,
    jasa_dibayar   BIGINT UNSIGNED NOT NULL DEFAULT 0,
    pokok_dibayar  BIGINT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uq_alokasi (id_pembayaran, id_angsuran),
    KEY ix_alokasi_angsuran (id_angsuran),
    CONSTRAINT fk_alokasi_pembayaran FOREIGN KEY (id_pembayaran) REFERENCES pembayaran_pinjaman (id),
    CONSTRAINT fk_alokasi_angsuran   FOREIGN KEY (id_angsuran)   REFERENCES angsuran (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------
-- log_audit: siapa mengubah apa dan kapan
-- ---------------------------------------------------------------------
CREATE TABLE log_audit (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    id_pengguna  BIGINT UNSIGNED NOT NULL,
    aksi         VARCHAR(50)     NOT NULL,
    entitas      VARCHAR(50)     NOT NULL,
    id_entitas   BIGINT UNSIGNED NOT NULL,
    detail       JSON            NULL,
    dibuat_pada  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY ix_audit_entitas (entitas, id_entitas),
    KEY ix_audit_pengguna (id_pengguna, dibuat_pada),
    CONSTRAINT fk_audit_pengguna FOREIGN KEY (id_pengguna) REFERENCES pengguna (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
