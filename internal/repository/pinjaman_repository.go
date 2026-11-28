package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type PinjamanRepo struct{}

// AdaAktif menjawab apakah anggota punya pinjaman berstatus aktif.
func (PinjamanRepo) AdaAktif(ctx context.Context, db database.DBTX, idAnggota int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM pinjaman WHERE id_anggota = ? AND status = 'aktif')`

	var ada bool
	err := db.QueryRowContext(ctx, query, idAnggota).Scan(&ada)

	return ada, petakanError(err, "cek pinjaman aktif")
}

// Create menyimpan pinjaman baru beserta seluruh baris jadwal angsuran dalam satu transaksi.
func (PinjamanRepo) Create(ctx context.Context, db database.DBTX, p model.Pinjaman, jadwal []model.Angsuran) (int64, error) {
	queryPinjaman := `
		INSERT INTO pinjaman (
			id_anggota, jumlah_pinjaman, tenor_bulan, jasa_per_bulan_bps,
			denda_per_hari, tanggal_pencairan, status, dibuat_oleh, dibuat_pada
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	res, err := db.ExecContext(ctx, queryPinjaman,
		p.IDAnggota,
		p.JumlahPinjaman,
		p.TenorBulan,
		p.JasaPerBulanBps,
		p.DendaPerHari,
		p.TanggalPencairan,
		p.Status,
		p.DibuatOleh,
		p.DibuatPada,
	)
	if err != nil {
		if dataDuplikat(err) {
			return 0, ErrDuplikat
		}
		return 0, petakanError(err, "simpan pinjaman")
	}

	idPinjaman, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("repository: last insert id pinjaman: %w", err)
	}

	queryAngsuran := `
		INSERT INTO angsuran (
			id_pinjaman, urutan, tanggal_jatuh_tempo, pokok_tagihan, jasa_tagihan
		) VALUES (?, ?, ?, ?, ?)
	`

	for _, a := range jadwal {
		_, err := db.ExecContext(ctx, queryAngsuran,
			idPinjaman,
			a.Urutan,
			a.JatuhTempo,
			a.Pokok,
			a.Jasa,
		)
		if err != nil {
			return 0, petakanError(err, fmt.Sprintf("simpan angsuran urutan %d", a.Urutan))
		}
	}

	return idPinjaman, nil
}

// GetByID mengambil satu data pinjaman berdasarkan ID beserta data anggotanya.
func (PinjamanRepo) GetByID(ctx context.Context, db database.DBTX, id int64) (model.PinjamanListItem, error) {
	query := `
		SELECT
			p.id, p.id_anggota, p.jumlah_pinjaman, p.tenor_bulan, p.jasa_per_bulan_bps,
			p.denda_per_hari, p.tanggal_pencairan, p.status, p.tanggal_lunas, p.dibuat_oleh, p.dibuat_pada,
			a.nomor_anggota, a.nama_lengkap
		FROM pinjaman p
		JOIN anggota a ON a.id = p.id_anggota
		WHERE p.id = ?
	`

	var item model.PinjamanListItem
	var tglLunas sql.NullTime

	err := db.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.IDAnggota,
		&item.JumlahPinjaman,
		&item.TenorBulan,
		&item.JasaPerBulanBps,
		&item.DendaPerHari,
		&item.TanggalPencairan,
		&item.Status,
		&tglLunas,
		&item.DibuatOleh,
		&item.DibuatPada,
		&item.NomorAnggota,
		&item.NamaAnggota,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PinjamanListItem{}, ErrTidakDitemukan
		}
		return model.PinjamanListItem{}, petakanError(err, "get pinjaman by id")
	}

	if tglLunas.Valid {
		item.TanggalLunas = &tglLunas.Time
	}

	return item, nil
}

// List mengambil daftar pinjaman, dapat difilter berdasarkan status (aktif, lunas, atau kosong untuk semua).
func (PinjamanRepo) List(ctx context.Context, db database.DBTX, status string) ([]model.PinjamanListItem, error) {
	query := `
		SELECT
			p.id, p.id_anggota, p.jumlah_pinjaman, p.tenor_bulan, p.jasa_per_bulan_bps,
			p.denda_per_hari, p.tanggal_pencairan, p.status, p.tanggal_lunas, p.dibuat_oleh, p.dibuat_pada,
			a.nomor_anggota, a.nama_lengkap
		FROM pinjaman p
		JOIN anggota a ON a.id = p.id_anggota
		WHERE (? = '' OR p.status = ?)
		ORDER BY p.id DESC
	`

	rows, err := db.QueryContext(ctx, query, status, status)
	if err != nil {
		return nil, petakanError(err, "list pinjaman")
	}
	defer rows.Close()

	var hasil []model.PinjamanListItem
	for rows.Next() {
		var item model.PinjamanListItem
		var tglLunas sql.NullTime

		err := rows.Scan(
			&item.ID,
			&item.IDAnggota,
			&item.JumlahPinjaman,
			&item.TenorBulan,
			&item.JasaPerBulanBps,
			&item.DendaPerHari,
			&item.TanggalPencairan,
			&item.Status,
			&tglLunas,
			&item.DibuatOleh,
			&item.DibuatPada,
			&item.NomorAnggota,
			&item.NamaAnggota,
		)
		if err != nil {
			return nil, petakanError(err, "scan pinjaman list item")
		}

		if tglLunas.Valid {
			item.TanggalLunas = &tglLunas.Time
		}
		hasil = append(hasil, item)
	}

	if err := rows.Err(); err != nil {
		return nil, petakanError(err, "iterasi pinjaman list")
	}

	return hasil, nil
}

// GetJadwalByPinjamanID mengambil jadwal angsuran suatu pinjaman terurut berdasarkan urutan.
func (PinjamanRepo) GetJadwalByPinjamanID(ctx context.Context, db database.DBTX, idPinjaman int64) ([]model.Angsuran, error) {
	query := `
		SELECT id, id_pinjaman, urutan, tanggal_jatuh_tempo, pokok_tagihan, jasa_tagihan
		FROM angsuran
		WHERE id_pinjaman = ?
		ORDER BY urutan ASC
	`

	rows, err := db.QueryContext(ctx, query, idPinjaman)
	if err != nil {
		return nil, petakanError(err, "get jadwal angsuran")
	}
	defer rows.Close()

	var jadwal []model.Angsuran
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
			return nil, petakanError(err, "scan angsuran")
		}
		jadwal = append(jadwal, a)
	}

	if err := rows.Err(); err != nil {
		return nil, petakanError(err, "iterasi jadwal angsuran")
	}

	return jadwal, nil
}

// GetStatusAngsuranByPinjamanID mengambil jadwal angsuran beserta realisasi pembayaran dan perhitungan denda terutang.
func (PinjamanRepo) GetStatusAngsuranByPinjamanID(ctx context.Context, db database.DBTX, idPinjaman int64, dendaPerHari int64, asOf time.Time) ([]model.StatusAngsuran, error) {
	query := `
		SELECT
			a.id, a.id_pinjaman, a.urutan, a.tanggal_jatuh_tempo, a.pokok_tagihan, a.jasa_tagihan,
			COALESCE(SUM(al.pokok_dibayar), 0) AS pokok_dibayar,
			COALESCE(SUM(al.jasa_dibayar), 0) AS jasa_dibayar,
			COALESCE(SUM(al.denda_dibayar), 0) AS denda_dibayar
		FROM angsuran a
		LEFT JOIN alokasi_pembayaran al ON al.id_angsuran = a.id
		LEFT JOIN pembayaran_pinjaman p ON p.id = al.id_pembayaran AND p.dibatalkan_pada IS NULL
		WHERE a.id_pinjaman = ?
		GROUP BY a.id, a.id_pinjaman, a.urutan, a.tanggal_jatuh_tempo, a.pokok_tagihan, a.jasa_tagihan
		ORDER BY a.urutan ASC
	`

	rows, err := db.QueryContext(ctx, query, idPinjaman)
	if err != nil {
		return nil, petakanError(err, "get status angsuran")
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
			&s.PokokDibayar,
			&s.JasaDibayar,
			&s.DendaDibayar,
		)
		if err != nil {
			return nil, petakanError(err, "scan status angsuran")
		}

		if !s.IsLunas() {
			dendaTotal := helper.HitungDenda(s.JatuhTempo, asOf, dendaPerHari)
			if sisaDenda := dendaTotal - s.DendaDibayar; sisaDenda > 0 {
				s.DendaTerutang = sisaDenda
			}
		}

		hasil = append(hasil, s)
	}

	if err := rows.Err(); err != nil {
		return nil, petakanError(err, "iterasi status angsuran")
	}

	return hasil, nil
}

// GetPembayaranByPinjamanID mengambil daftar pembayaran sah (tidak dibatalkan) untuk suatu pinjaman.
func (PinjamanRepo) GetPembayaranByPinjamanID(ctx context.Context, db database.DBTX, idPinjaman int64) ([]model.Pembayaran, error) {
	query := `
		SELECT id, id_pinjaman, jenis, tanggal_bayar, jumlah_diterima, biaya_administrasi
		FROM pembayaran_pinjaman
		WHERE id_pinjaman = ? AND dibatalkan_pada IS NULL
		ORDER BY tanggal_bayar ASC, id ASC
	`

	rows, err := db.QueryContext(ctx, query, idPinjaman)
	if err != nil {
		return nil, petakanError(err, "get pembayaran pinjaman")
	}
	defer rows.Close()

	var hasil []model.Pembayaran
	for rows.Next() {
		var p model.Pembayaran
		var jenis string
		err := rows.Scan(
			&p.ID,
			&p.IDPinjaman,
			&jenis,
			&p.TanggalBayar,
			&p.Jumlah,
			&p.BiayaAdmin,
		)
		if err != nil {
			return nil, petakanError(err, "scan pembayaran pinjaman")
		}
		p.Jenis = model.JenisPembayaran(jenis)
		hasil = append(hasil, p)
	}

	if err := rows.Err(); err != nil {
		return nil, petakanError(err, "iterasi pembayaran pinjaman")
	}

	return hasil, nil
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
