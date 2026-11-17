package repository

import (
	"context"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
)

type PinjamanRepo struct{}

// AdaAktif menjawab apakah anggota punya pinjaman berstatus aktif.
func (PinjamanRepo) AdaAktif(ctx context.Context, db database.DBTX, idAnggota int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM pinjaman WHERE id_anggota = ? AND status = 'aktif')`

	var ada bool
	err := db.QueryRowContext(ctx, query, idAnggota).Scan(&ada)

	return ada, petakanError(err, "cek pinjaman aktif")
}
