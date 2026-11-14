package repository

import (
	"context"
	"database/sql"
	"strings"

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

// GetAll mengembalikan semua anggota
func (AnggotaRepo) GetAll(ctx context.Context, db database.DBTX) ([]model.Anggota, error) {
	query := `SELECT id, nomor_anggota, nama_lengkap, no_telepon, tanggal_bergabung, status, tanggal_keluar
	FROM anggota ORDER BY nomor_anggota`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, petakanError(err, "GetAll anggota")
	}
	defer rows.Close()

	var hasil []model.Anggota
	for rows.Next() {
		a, _, err := scan(rows)
		if err != nil {
			return nil, petakanError(err, "scan anggota")
		}

		hasil = append(hasil, a)
	}

	return hasil, petakanError(rows.Err(), "GetAll anggota")
}

type filterClause struct {
	sql   string
	value interface{}
}

// ListFiltered mengembalikan list anggota yang dipaginasikan
func (AnggotaRepo) ListFiltered(ctx context.Context, db database.DBTX, cari, status string, page, limit int) ([]model.Anggota, int, error) {
	var listAnggota []model.Anggota
	offset := (page - 1) * limit

	query := `SELECT id, nomor_anggota, nama_lengkap, no_telepon, tanggal_bergabung, status, tanggal_keluar, COUNT(*) OVER() as total
	FROM anggota`

	args := []interface{}{}
	var clauses []string

	if cari != "" {
		clauses = append(clauses, `(nomor_anggota LIKE ? OR nama_lengkap LIKE ?)`)
		pola := `%` + cari + `%`
		args = append(args, pola, pola)
	}

	if status != "" {
		clauses = append(clauses, `status = ?`)
		args = append(args, status)
	}

	if len(clauses) > 0 {
		query += ` WHERE ` + strings.Join(clauses, ` AND `)
	}

	query += ` ORDER BY nomor_anggota DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, petakanError(err, "ListFiltered anggota")
	}
	defer rows.Close()

	var total int
	for rows.Next() {
		a, d, err := scan(rows)
		if err != nil {
			return nil, 0, petakanError(err, "scan anggota")
		}

		total = d
		listAnggota = append(listAnggota, a)
	}

	return listAnggota, total, rows.Err()
}

// HELPER //
func scan(rows *sql.Rows) (model.Anggota, int, error) {
	var (
		a       model.Anggota
		telepon sql.NullString
		keluar  sql.NullTime
		total   int
	)

	if err := rows.Scan(&a.ID, &a.NomorAnggota, &a.NamaLengkap, &telepon, &a.TanggalBergabung, &a.Status, &keluar, &total); err != nil {
		return model.Anggota{}, 0, err
	}

	a.NoTelepon = telepon.String
	if keluar.Valid {
		t := keluar.Time
		a.TanggalKeluar = &t
	}

	return a, total, nil
}
