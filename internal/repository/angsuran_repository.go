package repository

import (
	"context"
	"fmt"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type AngsuranRepo struct{}

func (AngsuranRepo) ListByPinjaman(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
) ([]model.Angsuran, error) {
	query := `SELECT
		id,
		id_pinjaman,
		urutan,
		tanggal_jatuh_tempo,
		pokok_tagihan,
		jasa_tagihan
		FROM angsuran
		WHERE id_pinjaman = ?
		ORDER BY urutan ASC`

	rows, err := db.QueryContext(ctx, query, idPinjaman)
	if err != nil {
		return nil, petakanError(err, "List angsuran pinjaman")
	}
	defer rows.Close()

	var hasil []model.Angsuran

	for rows.Next() {
		var a model.Angsuran

		err := rows.Scan(
			&a.ID,
			&a.IDPinjaman,
			&a.Urutan,
			&a.JatuhTempo,
			&a.Pokok,
			&a.Jasa,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca angsuran: %w", err)
		}

		hasil = append(hasil, a)
	}

	if err := rows.Err(); err != nil {
		return nil, petakanError(err, "List angsuran pinjaman")
	}

	return hasil, nil
}
