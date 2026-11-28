package repository

import (
	"context"
	"fmt"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type AlokasiRepository struct{}

func NewAlokasiRepository() *AlokasiRepository {
	return &AlokasiRepository{}
}

// Hapus seluruh alokasi pembayaran pada satu pinjaman.
// Digunakan sebelum replay alokasi pembayaran.
// Harus dipanggil di dalam transaksi database.
func (r *AlokasiRepository) DeleteByPinjaman(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
) error {
	query := `DELETE ap
		FROM alokasi_pembayaran ap
		JOIN pembayaran_pinjaman p ON p.id = ap.id_pembayaran
		WHERE p.id_pinjaman = ?`

	_, err := db.ExecContext(ctx, query, idPinjaman)
	if err != nil {
		return petakanError(err, "Delete alokasi pinjaman")
	}

	return nil
}

// CreateBatch menyimpan seluruh alokasi pembayaran.
// Harus dipanggil di dalam transaksi database.
func (r *AlokasiRepository) CreateBatch(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
	alokasi []model.Alokasi,
) error {
	for _, a := range alokasi {
		var idAngsuran int64

		err := db.QueryRowContext(
			ctx,
			`SELECT id
			FROM angsuran
			WHERE id_pinjaman = ? AND urutan = ?`,
			idPinjaman,
			a.Urutan,
		).Scan(&idAngsuran)

		if err != nil {
			return fmt.Errorf(
				"gagal mencari angsuran urutan %d: %w",
				a.Urutan,
				petakanErrorAngsuran(err),
			)
		}

		_, err = db.ExecContext(
			ctx,
			`INSERT INTO alokasi_pembayaran
			(id_pembayaran, id_angsuran, denda_dibayar, jasa_dibayar, pokok_dibayar)
			VALUES (?, ?, ?, ?, ?)`,
			a.IDPembayaran,
			idAngsuran,
			a.Denda,
			a.Jasa,
			a.Pokok,
		)

		if err != nil {
			return fmt.Errorf(
				"gagal menyimpan alokasi angsuran %d: %w",
				a.Urutan,
				petakanError(err, "Create alokasi"),
			)
		}
	}

	return nil
}

// petakanErrorAngsuran menerjemahkan error pencarian angsuran.
func petakanErrorAngsuran(err error) error {
	return petakanError(err, "Get angsuran untuk alokasi")
}
