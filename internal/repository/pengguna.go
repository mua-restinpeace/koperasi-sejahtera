package repository

import (
	"context"
	"errors"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

var (
	ErrPenggunaTidakDitemuan = errors.New("repository: pengguna tidak ditemukan")
)

type PenggunaRepo struct{}

// CariByUsername mengembalikan ErrUserTidakDitemukan bila username tidak ada
func (PenggunaRepo) CariByUsername(ctx context.Context, db database.DBTX, username string) (model.Pengguna, error) {
	var p model.Pengguna
	query := `SELECT id, username, password_hash, nama_lengkap, aktif FROM pengguna WHERE username = ?`

	err := db.QueryRowContext(ctx, query, username).Scan(&p.ID, &p.Username, &p.PasswordHash, &p.NamaLengkap, &p.Aktif)

	return p, petakanError(err, "CariByUsername")
}

// CariByID mengembalikan ErrTidakDitemukan bila ID tidak ada
func (PenggunaRepo) CariByID(ctx context.Context, db database.DBTX, id int64) (model.Pengguna, error) {
	var p model.Pengguna
	query := `SELECT id, username, password_hash, nama_lengkap, aktif FROM pengguna WHERE username = ?`

	err := db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Username, &p.PasswordHash, &p.NamaLengkap, &p.Aktif)

	return p, petakanError(err, "CariByID")
}
