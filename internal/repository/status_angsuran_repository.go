package repository

import (
	"context"
	"fmt"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type StatusAngsuranRepository struct{}

func NewStatusAngsuranRepository() *StatusAngsuranRepository {
	return &StatusAngsuranRepository{}
}

func (r *StatusAngsuranRepository) ListByPinjaman(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
) ([]model.StatusAngsuran, error) {
	query := `
		SELECT
			a.id,
			a.id_pinjaman,
			a.urutan,
			a.tanggal_jatuh_tempo,
			a.pokok_tagihan,
			a.jasa_tagihan,
			COALESCE(SUM(
				CASE WHEN p.id IS NOT NULL
				THEN ap.denda_dibayar ELSE 0 END
			), 0),
			COALESCE(SUM(
				CASE WHEN p.id IS NOT NULL
				THEN ap.jasa_dibayar ELSE 0 END
			), 0),
			COALESCE(SUM(
				CASE WHEN p.id IS NOT NULL
				THEN ap.pokok_dibayar ELSE 0 END
			), 0)
		FROM angsuran a
		LEFT JOIN alokasi_pembayaran ap
			ON ap.id_angsuran = a.id
		LEFT JOIN pembayaran_pinjaman p
			ON p.id = ap.id_pembayaran
			AND p.dibatalkan_pada IS NULL
		WHERE a.id_pinjaman = ?
		GROUP BY a.id
		ORDER BY a.urutan
	`

	rows, err := db.QueryContext(ctx, query, idPinjaman)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil status angsuran: %w", err)
	}
	defer rows.Close()

	var hasil []model.StatusAngsuran

	for rows.Next() {
		var s model.StatusAngsuran

		err := rows.Scan(
			&s.ID,
			&s.IDPinjaman,
			&s.Urutan,
			&s.JatuhTempo,
			&s.Pokok,
			&s.Jasa,
			&s.DendaDibayar,
			&s.JasaDibayar,
			&s.PokokDibayar,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca status angsuran: %w", err)
		}

		hasil = append(hasil, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gagal membaca data angsuran: %w", err)
	}

	return hasil, nil
}
