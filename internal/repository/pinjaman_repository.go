package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type PinjamanRepo struct{}

// GetByID mengambil data pinjaman berdasarkan ID.
func (PinjamanRepo) GetByID(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
) (model.Pinjaman, error) {
	query := `SELECT id, id_anggota, jumlah_pinjaman, tenor_bulan,
        jasa_per_bulan_bps, denda_per_hari, tanggal_pencairan,
        status, tanggal_lunas
        FROM pinjaman
        WHERE id = ?`

	var pinjaman model.Pinjaman
	var tanggalLunas sql.NullTime

	err := db.QueryRowContext(ctx, query, idPinjaman).Scan(
		&pinjaman.ID,
		&pinjaman.IDAnggota,
		&pinjaman.JumlahPinjaman,
		&pinjaman.TenorBulan,
		&pinjaman.JasaPerBulanBps,
		&pinjaman.DendaPerHari,
		&pinjaman.TanggalPencairan,
		&pinjaman.Status,
		&tanggalLunas,
	)
	if err != nil {
		return model.Pinjaman{}, petakanError(err, "ambil pinjaman")
	}

	if tanggalLunas.Valid {
		pinjaman.TanggalLunas = &tanggalLunas.Time
	}

	return pinjaman, nil
}

// AdaAktif menjawab apakah anggota punya pinjaman berstatus aktif.
func (PinjamanRepo) AdaAktif(ctx context.Context, db database.DBTX, idAnggota int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM pinjaman WHERE id_anggota = ? AND status = 'aktif')`

	var ada bool
	err := db.QueryRowContext(ctx, query, idAnggota).Scan(&ada)

	return ada, petakanError(err, "cek pinjaman aktif")
}

// KunciAktif mengambil dan mengunci baris pinjaman aktif.
// Harus dijalankan dalam transaksi database.
func (PinjamanRepo) KunciAktif(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
) error {
	var id int64

	query := `SELECT id
		FROM pinjaman
		WHERE id = ? AND status = 'aktif'
		FOR UPDATE`

	err := db.QueryRowContext(ctx, query, idPinjaman).Scan(&id)

	return petakanError(err, "Kunci pinjaman aktif")
}

// KunciByID mengunci pinjaman tanpa memeriksa statusnya.
// Digunakan untuk proses replay dan pembatalan.
func (PinjamanRepo) KunciByID(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
) error {
	var id int64

	query := `SELECT id
		FROM pinjaman
		WHERE id = ?
		FOR UPDATE`

	err := db.QueryRowContext(ctx, query, idPinjaman).Scan(&id)

	return petakanError(err, "Kunci pinjaman")
}

// UbahStatus memperbarui status pinjaman.
// Harus dipanggil di dalam transaksi database.
func (PinjamanRepo) UbahStatus(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
	status string,
	tanggalLunas *time.Time,
) error {
	query := `UPDATE pinjaman
        SET status = ?, tanggal_lunas = ?
        WHERE id = ?`

	var tanggal any
	if tanggalLunas != nil {
		tanggal = tanggalLunas.Format("2006-01-02")
	}

	_, err := db.ExecContext(
		ctx,
		query,
		status,
		tanggal,
		idPinjaman,
	)

	return petakanError(err, "Ubah status pinjaman")
}
