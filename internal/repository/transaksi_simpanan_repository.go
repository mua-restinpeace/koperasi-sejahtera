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

// Saldo menghitung saldo simpanan per kategori
func (SimpananRepo) Saldo(ctx context.Context, db database.DBTX, idAnggota int64) (model.SaldoSimpanan, error) {
	query := `SELECT kategori,
	CAST(COALESCE(SUM(CASE tipe_mutasi 
		WHEN 'setor' THEN CAST(jumlah AS SIGNED)
		ELSE -CAST(jumlah AS SIGNED)
	END), 0) AS SIGNED)
	FROM transaksi_simpanan
	WHERE id_anggota = ? AND dibatalkan_pada IS NULL
	GROUP BY kategori`

	rows, err := db.QueryContext(ctx, query, idAnggota)
	if err != nil {
		return model.SaldoSimpanan{}, petakanError(err, "Saldo simpanan")
	}
	defer rows.Close()

	var saldo model.SaldoSimpanan
	for rows.Next() {
		var (
			kategori string
			jumlah   int64
		)

		if err := rows.Scan(&kategori, &jumlah); err != nil {
			return model.SaldoSimpanan{}, petakanError(err, "scan Saldo simpanan")
		}

		switch kategori {
		case model.SimpananPokok:
			saldo.Pokok = jumlah
		case model.SimpananWajib:
			saldo.Wajib = jumlah
		case model.SimpananSukarela:
			saldo.Sukarela = jumlah
		}
	}

	return saldo, petakanError(rows.Err(), "Saldo simpanan")
}
