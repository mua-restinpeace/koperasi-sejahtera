package repository

import (
	"context"
	"strings"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

// LaporanRepo hanya membaca. Semua kueri membuang pembayaran yang sudah dibatalkan
// (dibatalkan_pada IS NOT NULL).
type LaporanRepo struct{}

const formatTanggal = "2006-01-02"

// rentang menambahkan syarat tanggal_bayar pada kolom kol; batas nil berarti terbuka.
func rentang(kol string, dari, sampai *time.Time) (string, []any) {
	var syarat []string
	var arg []any
	if dari != nil {
		syarat = append(syarat, kol+" >= ?")
		arg = append(arg, dari.Format(formatTanggal))
	}
	if sampai != nil {
		syarat = append(syarat, kol+" <= ?")
		arg = append(arg, sampai.Format(formatTanggal))
	}
	if len(syarat) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(syarat, " AND "), arg
}

// Pembayaran mengembalikan pembayaran yang belum dibatalkan pada rentang tanggal bayar.
func (LaporanRepo) Pembayaran(ctx context.Context, db database.DBTX, dari, sampai *time.Time) ([]model.PembayaranLaporan, error) {
	filter, arg := rentang("p.tanggal_bayar", dari, sampai)
	query := `SELECT p.id, p.tanggal_bayar, CAST(p.biaya_administrasi AS SIGNED)
	FROM pembayaran_pinjaman p
	WHERE p.dibatalkan_pada IS NULL` + filter + ` ORDER BY p.tanggal_bayar, p.id`

	rows, err := db.QueryContext(ctx, query, arg...)
	if err != nil {
		return nil, petakanError(err, "laporan pembayaran")
	}
	defer rows.Close()

	var hasil []model.PembayaranLaporan
	for rows.Next() {
		var b model.PembayaranLaporan
		if err := rows.Scan(&b.ID, &b.TanggalBayar, &b.BiayaAdmin); err != nil {
			return nil, petakanError(err, "scan laporan pembayaran")
		}
		hasil = append(hasil, b)
	}
	return hasil, petakanError(rows.Err(), "baca laporan pembayaran")
}

// Alokasi mengembalikan pembagian denda, jasa, dan pokok dari pembayaran yang belum
// dibatalkan pada rentang tanggal bayar.
func (LaporanRepo) Alokasi(ctx context.Context, db database.DBTX, dari, sampai *time.Time) ([]model.AlokasiLaporan, error) {
	filter, arg := rentang("p.tanggal_bayar", dari, sampai)
	query := `SELECT a.id_pembayaran, p.tanggal_bayar,
		CAST(a.denda_dibayar AS SIGNED), CAST(a.jasa_dibayar AS SIGNED), CAST(a.pokok_dibayar AS SIGNED)
	FROM alokasi_pembayaran a
	JOIN pembayaran_pinjaman p ON p.id = a.id_pembayaran
	WHERE p.dibatalkan_pada IS NULL` + filter + ` ORDER BY p.tanggal_bayar, a.id_pembayaran, a.id_angsuran`

	rows, err := db.QueryContext(ctx, query, arg...)
	if err != nil {
		return nil, petakanError(err, "laporan alokasi")
	}
	defer rows.Close()

	var hasil []model.AlokasiLaporan
	for rows.Next() {
		var a model.AlokasiLaporan
		if err := rows.Scan(&a.IDPembayaran, &a.TanggalBayar, &a.Denda, &a.Jasa, &a.Pokok); err != nil {
			return nil, petakanError(err, "scan laporan alokasi")
		}
		hasil = append(hasil, a)
	}
	return hasil, petakanError(rows.Err(), "baca laporan alokasi")
}

// pinjamanBerjalan adalah syarat "pinjaman yang masih berjalan pada tanggal asOf":
// sudah cair pada asOf, tidak dibatalkan, dan belum lunas pada asOf (berstatus aktif,
// atau lunasnya setelah asOf). Memakai dua parameter: asOf, asOf.
const pinjamanBerjalan = `pj.status <> 'dibatalkan' AND pj.tanggal_pencairan <= ?
	AND (pj.status = 'aktif' OR pj.tanggal_lunas > ?)`

// PinjamanBerjalan mengembalikan pinjaman yang masih berjalan pada asOf beserta anggotanya.
func (LaporanRepo) PinjamanBerjalan(ctx context.Context, db database.DBTX, asOf time.Time) ([]model.PinjamanLaporan, error) {
	tgl := asOf.Format(formatTanggal)
	query := `SELECT pj.id, pj.id_anggota, a.nomor_anggota, a.nama_lengkap, CAST(pj.denda_per_hari AS SIGNED)
	FROM pinjaman pj
	JOIN anggota a ON a.id = pj.id_anggota
	WHERE ` + pinjamanBerjalan + ` ORDER BY pj.id`

	rows, err := db.QueryContext(ctx, query, tgl, tgl)
	if err != nil {
		return nil, petakanError(err, "laporan pinjaman berjalan")
	}
	defer rows.Close()

	var hasil []model.PinjamanLaporan
	for rows.Next() {
		var p model.PinjamanLaporan
		if err := rows.Scan(&p.ID, &p.IDAnggota, &p.NomorAnggota, &p.NamaAnggota, &p.DendaPerHari); err != nil {
			return nil, petakanError(err, "scan laporan pinjaman")
		}
		hasil = append(hasil, p)
	}
	return hasil, petakanError(rows.Err(), "baca laporan pinjaman")
}

// AngsuranBerjalan mengembalikan SELURUH jadwal angsuran pinjaman yang berjalan pada
// asOf (helper.HitungStatus butuh jadwal lengkap).
func (LaporanRepo) AngsuranBerjalan(ctx context.Context, db database.DBTX, asOf time.Time) ([]model.Angsuran, error) {
	tgl := asOf.Format(formatTanggal)
	query := `SELECT g.id, g.id_pinjaman, g.urutan, g.tanggal_jatuh_tempo,
		CAST(g.pokok_tagihan AS SIGNED), CAST(g.jasa_tagihan AS SIGNED)
	FROM angsuran g
	JOIN pinjaman pj ON pj.id = g.id_pinjaman
	WHERE ` + pinjamanBerjalan + ` ORDER BY g.id_pinjaman, g.urutan`

	rows, err := db.QueryContext(ctx, query, tgl, tgl)
	if err != nil {
		return nil, petakanError(err, "laporan angsuran berjalan")
	}
	defer rows.Close()

	var hasil []model.Angsuran
	for rows.Next() {
		var a model.Angsuran
		if err := rows.Scan(&a.ID, &a.IDPinjaman, &a.Urutan, &a.JatuhTempo, &a.Pokok, &a.Jasa); err != nil {
			return nil, petakanError(err, "scan laporan angsuran")
		}
		hasil = append(hasil, a)
	}
	return hasil, petakanError(rows.Err(), "baca laporan angsuran")
}

// PembayaranBerjalan mengembalikan pembayaran yang belum dibatalkan dan sudah terjadi
// pada atau sebelum asOf, untuk pinjaman yang berjalan pada asOf. Terurut menurut
// (tanggal_bayar, id), urutan yang dipakai replay alokasi.
func (LaporanRepo) PembayaranBerjalan(ctx context.Context, db database.DBTX, asOf time.Time) ([]model.Pembayaran, error) {
	tgl := asOf.Format(formatTanggal)
	query := `SELECT p.id, p.id_pinjaman, p.jenis, p.tanggal_bayar,
		CAST(p.jumlah_diterima AS SIGNED), CAST(p.biaya_administrasi AS SIGNED)
	FROM pembayaran_pinjaman p
	JOIN pinjaman pj ON pj.id = p.id_pinjaman
	WHERE p.dibatalkan_pada IS NULL AND p.tanggal_bayar <= ? AND ` + pinjamanBerjalan + `
	ORDER BY p.tanggal_bayar, p.id`

	rows, err := db.QueryContext(ctx, query, tgl, tgl, tgl)
	if err != nil {
		return nil, petakanError(err, "laporan pembayaran berjalan")
	}
	defer rows.Close()

	var hasil []model.Pembayaran
	for rows.Next() {
		var p model.Pembayaran
		if err := rows.Scan(&p.ID, &p.IDPinjaman, &p.Jenis, &p.TanggalBayar, &p.Jumlah, &p.BiayaAdmin); err != nil {
			return nil, petakanError(err, "scan laporan pembayaran")
		}
		hasil = append(hasil, p)
	}
	return hasil, petakanError(rows.Err(), "baca laporan pembayaran")
}
