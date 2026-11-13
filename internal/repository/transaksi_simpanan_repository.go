package repository

import (
	"context"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type SimpananRepo struct{}

// Create menyimpan satu transaksi simpanan dan mengembalikan ID nya
func (SimpananRepo) Create(ctx context.Context, db database.DBTX, t model.TransaksiSimpanan) (int64, error) {
	var periode, ket any
	if t.Periode != nil {
		periode = t.Periode.Format("2006-01-02")
	}

	if t.Keterangan != "" {
		ket = t.Keterangan
	}

	query := `INSERT INTO transaksi_simpanan (id_anggota, kategori, tipe_mutasi, jumlah, periode, tanggal_transaksi, keterangan, dibuat_oleh, dibuat_pada)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, query, t.IDAnggota, t.Kategori, t.TipeMutasi, t.Jumlah, periode, t.TanggalTransaksi.Format("2006-01-02"), ket, t.DibuatOleh, time.Now().UTC())

	if dataDuplikat(err) {
		return 0, ErrDuplikat
	}

	if err != nil {
		return 0, petakanError(err, "create transaksi simpanan")
	}

	id, err := res.LastInsertId()
	return id, petakanError(err, "id transaksi simpanan")
}
