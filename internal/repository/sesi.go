package repository

import (
	"context"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type SesiRepo struct{}

func (SesiRepo) Create(ctx context.Context, db database.DBTX, s model.Sesi) error {
	query := `INSERT INTO sesi (id_pengguna, token_hash, csrf_token, dibuat_pada, kedaluwarsa_pada)
	VALUES (?, ?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query, s.IDPengguna, s.TokenHash, s.CSRFToken, s.DibuatPada.UTC(), s.KedaluwarsaPada.UTC())
	return petakanError(err, "create sesi")
}

// CariAktif mengembalikan sesi beserta pengguna aktif pemiliknya bila token dikenal dan belum kedaluwarsa
func (SesiRepo) CariAktif(ctx context.Context, db database.DBTX, tokenHash string, sekarang time.Time) (model.Sesi, model.Pengguna, error) {
	var (
		s model.Sesi
		p model.Pengguna
	)

	query := `SELECT s.id, s.id_pengguna, s.token_hash, s.csrf_token, s.dibuat_pada, s.kedaluwarsa_pada, p.id, p.username, p.password_hash, p.nama_lengkap, p.aktif
	FROM sesi s join pengguna p
	ON p.id = s.id_pengguna
	WHERE s.token_hash = ? AND s.kedaluwarsa_pada > ? AND p.aktif = 1`

	err := db.QueryRowContext(ctx, query, tokenHash, sekarang.UTC()).Scan(&s.ID, &s.IDPengguna, &s.TokenHash, &s.CSRFToken, &s.DibuatPada, &s.KedaluwarsaPada, &p.ID, &p.Username, &p.PasswordHash, &p.NamaLengkap, &p.Aktif)

	return s, p, petakanError(err, "cari sesi")
}

// HapusByTokenHash menghapus satu sesi (logout). Tidak error bila sesi tidak ada.
func (SesiRepo) HapusByTokenHash(ctx context.Context, db database.DBTX, tokenHash string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sesi WHERE token_hash = ?`, tokenHash)
	return petakanError(err, "hapus sesi")
}

// HapusKedaluwarsa membersihkan sesi yang sudah lewat waktu
func (SesiRepo) HapusKedaluwarsa(ctx context.Context, db database.DBTX, sekarang time.Time) error {
	query := `DELETE FROM sesi WHERE kedaluwarsa_pada <= ?`
	_, err := db.ExecContext(ctx, query, sekarang.UTC())
	return petakanError(err, "hapus sesi kedaluwarsa")
}
