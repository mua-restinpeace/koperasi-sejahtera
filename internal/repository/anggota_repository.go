package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

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
		a, err := scanAnggota(rows, &total)
		if err != nil {
			return nil, 0, petakanError(err, "scan anggota")
		}

		listAnggota = append(listAnggota, a)
	}

	return listAnggota, total, petakanError(rows.Err(), "ListFiltered anggota")
}

// GetByID mengambil satu anggota
func (AnggotaRepo) GetByID(ctx context.Context, db database.DBTX, id int64) (model.Anggota, error) {
	query := `SELECT id, nomor_anggota, nama_lengkap, no_telepon, tanggal_bergabung, status, tanggal_keluar FROM anggota
	WHERE id = ?`
	return scanOne(ctx, db, query, id, "GetByID anggota")
}

// GetByIDForUpdate sama dengan GetByID, tetapi baris anggota dikunci sampai
// transaksi selesai (SELECT ... FOR UPDATE). Hanya bermakna bila db adalah
// transaksi; dipakai sebelum mengubah anggota atau datanya supaya dua pengurus
// tidak saling menimpa.
func (AnggotaRepo) GetByIDForUpdate(ctx context.Context, db database.DBTX, id int64) (model.Anggota, error) {
	return scanOne(ctx, db, `SELECT id, nomor_anggota, nama_lengkap, no_telepon, tanggal_bergabung, status, tanggal_keluar FROM anggota WHERE id = ? FOR UPDATE`, id, "GetByIDForUpdate anggota")
}

// Update hanya dapat mengubah nama dan telepon.
func (AnggotaRepo) Update(ctx context.Context, db database.DBTX, id int64, nama, telepon string) error {
	query := `UPDATE anggota set nama_lengkap = ?, no_telepon = ? WHERE id = ?`

	_, err := db.ExecContext(ctx, query, nama, nullBilaKosong(telepon), id)
	return petakanError(err, "Update anggota")
}

// Deactiveate mengubah status anggota aktif menjadi keluar.
func (AnggotaRepo) Deactivate(ctx context.Context, db database.DBTX, id int64, tanggal time.Time) error {
	query := `UPDATE anggota SET status = 'keluar', tanggal_keluar = ?
	WHERE id = ? AND status = 'aktif'`

	res, err := db.ExecContext(ctx, query, tanggal.Format("2006-01-02"), id)
	if err != nil {
		return petakanError(err, "Deactivate anggota")
	}

	n, err := res.RowsAffected()
	if err != nil {
		return petakanError(err, "Deactivate anggota")
	}

	if n == 0 {
		return ErrTidakDitemukan
	}

	return nil
}

// HELPER //
// pemindai adalah bagian dari *sql.Row dan *sql.Rows yang dipakai untuk membaca satu baris.
type pemindai interface {
	Scan(dest ...any) error
}

// bacaSatu menjalankan query yang menerima satu parameter ID dan membaca satu anggota.
func scanOne(ctx context.Context, db database.DBTX, query string, id int64, konteks string) (model.Anggota, error) {
	a, err := scanAnggota(db.QueryRowContext(ctx, query, id), nil)
	if err != nil {
		return model.Anggota{}, petakanError(err, konteks)
	}

	return a, nil
}

// scanAnggota membaca satu baris anggota (kolom sesuai kolomAnggota). Bila total tidak nil,
// kolom tambahan COUNT(*) OVER() di ujung baris ikut dibaca ke sana.
func scanAnggota(p pemindai, total *int) (model.Anggota, error) {
	var (
		a       model.Anggota
		telepon sql.NullString
		keluar  sql.NullTime
	)

	dest := []any{&a.ID, &a.NomorAnggota, &a.NamaLengkap, &telepon, &a.TanggalBergabung, &a.Status, &keluar}
	if total != nil {
		dest = append(dest, total)
	}

	if err := p.Scan(dest...); err != nil {
		return model.Anggota{}, err
	}

	a.NoTelepon = telepon.String
	if keluar.Valid {
		t := keluar.Time
		a.TanggalKeluar = &t
	}

	return a, nil
}

// nullBilaKosong mengubah string kosong menjadi NULL untuk kolom yang boleh NULL.
func nullBilaKosong(s string) any {
	if s == "" {
		return nil
	}

	return s
}
