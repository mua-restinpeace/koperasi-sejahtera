package repository

import (
	"context"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type AnggotaRepo struct{}

// Create menyimpan anggota baru dan mengembalikan ID nya
func (AnggotaRepo) Create(ctx context.Context, db database.DBTX, a model.Anggota) (int64, error) {
	query := `INSERT INTO anggota (nomor_anggota, nama_lengkap, no_telepon, tanggal_bergabung, status, dibuat_pada)
	VALUES  (?, ?, ?, ?, 'aktif', UTC_TIMESTAMP())`

	res, err := db.ExecContext(ctx, query, a.NomorAnggota, a.NamaLengkap, a.NoTelepon, a.TanggalBergabung.Format("2006-01-02"))

	if dataDuplikat(err) {
		return 0, ErrDuplikat
	}

	if err != nil {
		return 0, petakanError(err, "create anggota")
	}

	id, err := res.LastInsertId()
	return id, petakanError(err, "id anggota")
}
