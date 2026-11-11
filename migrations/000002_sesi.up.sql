-- Sesi login pengurus. Token asli hanya ada di cookie pengguna; yang disimpan
-- di sini hanya hash SHA-256-nya, sehingga bocornya tabel ini tidak membocorkan sesi.
-- Waktu disimpan sebagai DATETIME dalam UTC (diisi dari aplikasi).
CREATE TABLE sesi (
    id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    id_pengguna      BIGINT UNSIGNED NOT NULL,
    token_hash       CHAR(64)        NOT NULL COMMENT 'SHA-256 heksadesimal dari token pada cookie',
    csrf_token       CHAR(43)        NOT NULL COMMENT 'token anti-CSRF untuk form pada sesi ini',
    dibuat_pada      DATETIME        NOT NULL,
    kedaluwarsa_pada DATETIME        NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sesi_token (token_hash),
    KEY ix_sesi_pengguna (id_pengguna),
    KEY ix_sesi_kedaluwarsa (kedaluwarsa_pada),
    CONSTRAINT fk_sesi_pengguna FOREIGN KEY (id_pengguna) REFERENCES pengguna (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
