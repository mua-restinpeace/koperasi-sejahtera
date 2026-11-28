package repository

import (
	"context"
	"database/sql"
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

const kolomPinjamanKartu = `pj.id, pj.id_anggota, a.nomor_anggota, a.nama_lengkap,
	CAST(pj.jumlah_pinjaman AS SIGNED), pj.tenor_bulan, pj.jasa_per_bulan_bps,
	CAST(pj.denda_per_hari AS SIGNED), pj.tanggal_pencairan, pj.status, pj.tanggal_lunas`

type pemindai interface{ Scan(dest ...any) error }

func pindaiPinjamanKartu(r pemindai) (model.PinjamanKartu, error) {
	var p model.PinjamanKartu
	var lunas sql.NullTime
	err := r.Scan(&p.ID, &p.IDAnggota, &p.NomorAnggota, &p.NamaAnggota, &p.Jumlah, &p.TenorBulan,
		&p.JasaBps, &p.DendaPerHari, &p.TanggalPencairan, &p.Status, &lunas)
	if lunas.Valid {
		p.TanggalLunas = lunas.Time
	}
	return p, err
}

// DaftarPinjaman mengembalikan semua pinjaman yang tidak dibatalkan, terbaru di atas.
func (LaporanRepo) DaftarPinjaman(ctx context.Context, db database.DBTX) ([]model.PinjamanKartu, error) {
	query := `SELECT ` + kolomPinjamanKartu + `
	FROM pinjaman pj JOIN anggota a ON a.id = pj.id_anggota
	WHERE pj.status <> 'dibatalkan' ORDER BY pj.id DESC`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, petakanError(err, "daftar pinjaman")
	}
	defer rows.Close()

	var hasil []model.PinjamanKartu
	for rows.Next() {
		p, err := pindaiPinjamanKartu(rows)
		if err != nil {
			return nil, petakanError(err, "scan daftar pinjaman")
		}
		hasil = append(hasil, p)
	}
	return hasil, petakanError(rows.Err(), "baca daftar pinjaman")
}

// DetailPinjaman mengembalikan satu pinjaman; ErrTidakDitemukan bila tidak ada
// atau sudah dibatalkan.
func (LaporanRepo) DetailPinjaman(ctx context.Context, db database.DBTX, id int64) (model.PinjamanKartu, error) {
	query := `SELECT ` + kolomPinjamanKartu + `
	FROM pinjaman pj JOIN anggota a ON a.id = pj.id_anggota
	WHERE pj.id = ? AND pj.status <> 'dibatalkan'`

	p, err := pindaiPinjamanKartu(db.QueryRowContext(ctx, query, id))
	return p, petakanError(err, "detail pinjaman")
}

// AngsuranPinjaman mengembalikan seluruh jadwal angsuran satu pinjaman.
func (LaporanRepo) AngsuranPinjaman(ctx context.Context, db database.DBTX, id int64) ([]model.Angsuran, error) {
	query := `SELECT id, id_pinjaman, urutan, tanggal_jatuh_tempo,
		CAST(pokok_tagihan AS SIGNED), CAST(jasa_tagihan AS SIGNED)
	FROM angsuran WHERE id_pinjaman = ? ORDER BY urutan`

	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, petakanError(err, "angsuran pinjaman")
	}
	defer rows.Close()

	var hasil []model.Angsuran
	for rows.Next() {
		var a model.Angsuran
		if err := rows.Scan(&a.ID, &a.IDPinjaman, &a.Urutan, &a.JatuhTempo, &a.Pokok, &a.Jasa); err != nil {
			return nil, petakanError(err, "scan angsuran pinjaman")
		}
		hasil = append(hasil, a)
	}
	return hasil, petakanError(rows.Err(), "baca angsuran pinjaman")
}

// PembayaranPinjaman mengembalikan pembayaran satu pinjaman yang belum dibatalkan,
// terurut menurut (tanggal_bayar, id) seperti yang dipakai replay alokasi.
func (LaporanRepo) PembayaranPinjaman(ctx context.Context, db database.DBTX, id int64) ([]model.Pembayaran, error) {
	query := `SELECT id, id_pinjaman, jenis, tanggal_bayar,
		CAST(jumlah_diterima AS SIGNED), CAST(biaya_administrasi AS SIGNED)
	FROM pembayaran_pinjaman
	WHERE id_pinjaman = ? AND dibatalkan_pada IS NULL ORDER BY tanggal_bayar, id`

	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, petakanError(err, "pembayaran pinjaman")
	}
	defer rows.Close()

	var hasil []model.Pembayaran
	for rows.Next() {
		var p model.Pembayaran
		if err := rows.Scan(&p.ID, &p.IDPinjaman, &p.Jenis, &p.TanggalBayar, &p.Jumlah, &p.BiayaAdmin); err != nil {
			return nil, petakanError(err, "scan pembayaran pinjaman")
		}
		hasil = append(hasil, p)
	}
	return hasil, petakanError(rows.Err(), "baca pembayaran pinjaman")
}

// RingkasanAnggota menghitung anggota aktif dan yang bergabung pada bulan berjalan
// (dari awalBulan sampai hariIni).
func (LaporanRepo) RingkasanAnggota(ctx context.Context, db database.DBTX, awalBulan, hariIni time.Time) (aktif, baru int, err error) {
	query := `SELECT COUNT(*),
		CAST(COALESCE(SUM(tanggal_bergabung >= ? AND tanggal_bergabung <= ?), 0) AS SIGNED)
	FROM anggota WHERE status = 'aktif'`

	err = db.QueryRowContext(ctx, query, awalBulan.Format(formatTanggal), hariIni.Format(formatTanggal)).Scan(&aktif, &baru)
	return aktif, baru, petakanError(err, "ringkasan anggota")
}

// TotalSimpanan menjumlahkan seluruh simpanan (setor dikurangi tarik) yang belum dibatalkan.
func (LaporanRepo) TotalSimpanan(ctx context.Context, db database.DBTX) (int64, error) {
	query := `SELECT CAST(COALESCE(SUM(
		CASE WHEN tipe_mutasi = 'setor' THEN CAST(jumlah AS SIGNED) ELSE -CAST(jumlah AS SIGNED) END
	), 0) AS SIGNED)
	FROM transaksi_simpanan WHERE dibatalkan_pada IS NULL`

	var total int64
	err := db.QueryRowContext(ctx, query).Scan(&total)
	return total, petakanError(err, "total simpanan")
}

// WajibBelum mengembalikan paling banyak batas anggota aktif yang belum menyetor
// simpanan wajib untuk periode (tanggal 1 bulan itu), beserta jumlah seluruhnya.
func (LaporanRepo) WajibBelum(ctx context.Context, db database.DBTX, periode, hariIni time.Time, batas int) ([]model.AnggotaWajibBelum, int, error) {
	const dari = `FROM anggota a
	WHERE a.status = 'aktif' AND a.tanggal_bergabung <= ?
	AND NOT EXISTS (
		SELECT 1 FROM transaksi_simpanan t
		WHERE t.id_anggota = a.id AND t.kategori = 'wajib' AND t.tipe_mutasi = 'setor'
		AND t.dibatalkan_pada IS NULL AND t.periode = ?
	)`
	tgl, per := hariIni.Format(formatTanggal), periode.Format(formatTanggal)

	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) `+dari, tgl, per).Scan(&total); err != nil {
		return nil, 0, petakanError(err, "hitung wajib belum")
	}

	rows, err := db.QueryContext(ctx, `SELECT a.id, a.nomor_anggota, a.nama_lengkap `+dari+` ORDER BY a.nomor_anggota LIMIT ?`, tgl, per, batas)
	if err != nil {
		return nil, 0, petakanError(err, "daftar wajib belum")
	}
	defer rows.Close()

	var hasil []model.AnggotaWajibBelum
	for rows.Next() {
		var a model.AnggotaWajibBelum
		if err := rows.Scan(&a.ID, &a.NomorAnggota, &a.NamaAnggota); err != nil {
			return nil, 0, petakanError(err, "scan wajib belum")
		}
		hasil = append(hasil, a)
	}
	return hasil, total, petakanError(rows.Err(), "baca wajib belum")
}

// WajibBelumPeriode mengembalikan semua anggota yang pada periode itu (tanggal 1 bulan
// yang dibayar) sudah menjadi anggota, belum keluar sampai akhir bulan, dan belum
// menyetor simpanan wajib untuk periode tersebut. Setoran yang dibatalkan dianggap belum bayar.
func (LaporanRepo) WajibBelumPeriode(ctx context.Context, db database.DBTX, periode time.Time) ([]model.AnggotaWajibBelum, error) {
	akhir := time.Date(periode.Year(), periode.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	query := `SELECT a.id, a.nomor_anggota, a.nama_lengkap
	FROM anggota a
	WHERE a.tanggal_bergabung <= ? AND (a.tanggal_keluar IS NULL OR a.tanggal_keluar > ?)
	AND NOT EXISTS (
		SELECT 1 FROM transaksi_simpanan t
		WHERE t.id_anggota = a.id AND t.kategori = 'wajib' AND t.tipe_mutasi = 'setor'
		AND t.dibatalkan_pada IS NULL AND t.periode = ?
	)
	ORDER BY a.nomor_anggota`

	rows, err := db.QueryContext(ctx, query, akhir.Format(formatTanggal), akhir.Format(formatTanggal), periode.Format(formatTanggal))
	if err != nil {
		return nil, petakanError(err, "wajib belum per periode")
	}
	defer rows.Close()

	var hasil []model.AnggotaWajibBelum
	for rows.Next() {
		var a model.AnggotaWajibBelum
		if err := rows.Scan(&a.ID, &a.NomorAnggota, &a.NamaAnggota); err != nil {
			return nil, petakanError(err, "scan wajib belum per periode")
		}
		hasil = append(hasil, a)
	}
	return hasil, petakanError(rows.Err(), "baca wajib belum per periode")
}
