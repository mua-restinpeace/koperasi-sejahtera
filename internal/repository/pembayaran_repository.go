package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type PembayaranRepo struct{}

// Create menyimpan pembayaran dan mengembalikan ID.
func (PembayaranRepo) Create(
	ctx context.Context,
	db database.DBTX,
	p model.Pembayaran,
	dibuatOleh int64,
	keterangan string,
) (int64, error) {
	var ket any
	if keterangan != "" {
		ket = keterangan
	}

	query := `INSERT INTO pembayaran_pinjaman
		(id_pinjaman, jenis, tanggal_bayar, jumlah_diterima,
		 biaya_administrasi, keterangan, dibuat_oleh)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	res, err := db.ExecContext(ctx, query,
		p.IDPinjaman,
		p.Jenis,
		p.TanggalBayar.Format("2006-01-02"),
		p.Jumlah,
		p.BiayaAdmin,
		ket,
		dibuatOleh,
	)
	if err != nil {
		return 0, petakanError(err, "create pembayaran")
	}

	id, err := res.LastInsertId()
	return id, petakanError(err, "id pembayaran")
}

// ListAktifByPinjaman mengambil pembayaran yang belum dibatalkan.
func (PembayaranRepo) ListAktifByPinjaman(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
) ([]model.Pembayaran, error) {
	query := `SELECT id, id_pinjaman, jenis, tanggal_bayar,
		jumlah_diterima, biaya_administrasi
		FROM pembayaran_pinjaman
		WHERE id_pinjaman = ? AND dibatalkan_pada IS NULL
		ORDER BY tanggal_bayar ASC, id ASC`

	rows, err := db.QueryContext(ctx, query, idPinjaman)
	if err != nil {
		return nil, petakanError(err, "List pembayaran")
	}
	defer rows.Close()

	var hasil []model.Pembayaran
	for rows.Next() {
		p, err := scanPembayaran(rows)
		if err != nil {
			return nil, petakanError(err, "scan pembayaran")
		}
		hasil = append(hasil, p)
	}

	return hasil, petakanError(rows.Err(), "List pembayaran")
}

// ListRiwayatByPinjaman mengambil seluruh riwayat pembayaran,
// termasuk transaksi yang sudah dibatalkan.
func (PembayaranRepo) ListRiwayatByPinjaman(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
) ([]model.Pembayaran, error) {
	query := `SELECT id, id_pinjaman, jenis, tanggal_bayar,
        jumlah_diterima, biaya_administrasi, keterangan,
        dibatalkan_pada, alasan_pembatalan
        FROM pembayaran_pinjaman
        WHERE id_pinjaman = ?
        ORDER BY tanggal_bayar DESC, id DESC`

	rows, err := db.QueryContext(ctx, query, idPinjaman)
	if err != nil {
		return nil, petakanError(err, "List riwayat pembayaran")
	}
	defer rows.Close()

	var hasil []model.Pembayaran

	for rows.Next() {
		var p model.Pembayaran
		var keterangan sql.NullString
		var dibatalkanPada sql.NullTime
		var alasan sql.NullString

		err := rows.Scan(
			&p.ID,
			&p.IDPinjaman,
			&p.Jenis,
			&p.TanggalBayar,
			&p.Jumlah,
			&p.BiayaAdmin,
			&keterangan,
			&dibatalkanPada,
			&alasan,
		)
		if err != nil {
			return nil, petakanError(err, "scan riwayat pembayaran")
		}

		if keterangan.Valid {
			p.Keterangan = keterangan.String
		}

		if dibatalkanPada.Valid {
			p.DibatalkanPada = &dibatalkanPada.Time
		}

		if alasan.Valid {
			p.AlasanPembatalan = alasan.String
		}

		hasil = append(hasil, p)
	}

	return hasil, petakanError(rows.Err(), "List riwayat pembayaran")
}

// GetByIDForUpdate mengambil pembayaran dan mengunci barisnya.
func (PembayaranRepo) GetByIDForUpdate(
	ctx context.Context,
	db database.DBTX,
	id int64,
) (model.Pembayaran, error) {
	query := `SELECT id, id_pinjaman, jenis, tanggal_bayar,
		jumlah_diterima, biaya_administrasi
		FROM pembayaran_pinjaman
		WHERE id = ? FOR UPDATE`

	row := db.QueryRowContext(ctx, query, id)
	p, err := scanPembayaran(row)

	return p, petakanError(err, "GetByIDForUpdate pembayaran")
}

func scanPembayaran(row scanner) (model.Pembayaran, error) {
	var p model.Pembayaran

	err := row.Scan(
		&p.ID,
		&p.IDPinjaman,
		&p.Jenis,
		&p.TanggalBayar,
		&p.Jumlah,
		&p.BiayaAdmin,
	)
	if err != nil {
		return model.Pembayaran{}, err
	}

	return p, nil
}

// Batalkan menandai pembayaran dibatalkan tanpa menghapus data.
func (PembayaranRepo) Batalkan(
	ctx context.Context,
	db database.DBTX,
	id int64,
	oleh int64,
	alasan string,
	pada time.Time,
) error {
	query := `UPDATE pembayaran_pinjaman
		SET dibatalkan_pada = ?,
			dibatalkan_oleh = ?,
			alasan_pembatalan = ?
		WHERE id = ? AND dibatalkan_pada IS NULL`

	res, err := db.ExecContext(ctx, query, pada, oleh, alasan, id)
	if err != nil {
		return petakanError(err, "Batalkan pembayaran")
	}

	n, err := res.RowsAffected()
	if err != nil {
		return petakanError(err, "RowsAffected Batalkan pembayaran")
	}

	if n == 0 {
		return ErrTidakDitemukan
	}

	return nil
}
