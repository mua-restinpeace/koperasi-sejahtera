package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type LaporanRepo interface {
	Pembayaran(ctx context.Context, db database.DBTX, dari, sampai *time.Time) ([]model.PembayaranLaporan, error)
	Alokasi(ctx context.Context, db database.DBTX, dari, sampai *time.Time) ([]model.AlokasiLaporan, error)
	PinjamanBerjalan(ctx context.Context, db database.DBTX, asOf time.Time) ([]model.PinjamanLaporan, error)
	AngsuranBerjalan(ctx context.Context, db database.DBTX, asOf time.Time) ([]model.Angsuran, error)
	PembayaranBerjalan(ctx context.Context, db database.DBTX, asOf time.Time) ([]model.Pembayaran, error)

	// L-1 kartu pinjaman
	DaftarPinjaman(ctx context.Context, db database.DBTX) ([]model.PinjamanKartu, error)
	DetailPinjaman(ctx context.Context, db database.DBTX, id int64) (model.PinjamanKartu, error)
	AngsuranPinjaman(ctx context.Context, db database.DBTX, id int64) ([]model.Angsuran, error)
	PembayaranPinjaman(ctx context.Context, db database.DBTX, id int64) ([]model.Pembayaran, error)

	// Dashboard
	RingkasanAnggota(ctx context.Context, db database.DBTX, awalBulan, hariIni time.Time) (aktif, baru int, err error)
	TotalSimpanan(ctx context.Context, db database.DBTX) (int64, error)
	WajibBelum(ctx context.Context, db database.DBTX, periode, hariIni time.Time, batas int) ([]model.AnggotaWajibBelum, int, error)

	// L-4 simpanan wajib belum dibayar
	WajibBelumPeriode(ctx context.Context, db database.DBTX, periode time.Time) ([]model.AnggotaWajibBelum, error)
}

// LaporanService menyusun laporan (hanya baca). Rumusnya ada di fungsi murni
// HitungPendapatan dan HitungTunggakan di berkas ini, supaya bisa diuji tanpa basis data.
type LaporanService struct {
	db            database.Sumber
	repo          LaporanRepo
	biayaAdminBps int
	zona          *time.Location
	now           func() time.Time
	wajibPerBulan int64 // nominal simpanan wajib, untuk kolom "kekurangan" di dashboard
}

func NewLaporanService(db database.Sumber, repo LaporanRepo, biayaAdminBps int, zona *time.Location, now func() time.Time) *LaporanService {
	if now == nil {
		now = time.Now
	}

	if zona == nil {
		zona = time.UTC
	}

	return &LaporanService{db: db, repo: repo, biayaAdminBps: biayaAdminBps, zona: zona, now: now}
}

func (s *LaporanService) hariIni() time.Time {
	n := s.now().In(s.zona)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

var namaBulan = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// =====================================================================
// L-3 Pendapatan bulanan
// =====================================================================

// PendapatanBulan adalah pendapatan koperasi pada satu bulan (basis kas).
type PendapatanBulan struct {
	Bulan      time.Time // tanggal 1 bulan yang bersangkutan
	Jasa       int64
	Denda      int64
	BiayaAdmin int64
}

func (p PendapatanBulan) Total() int64 { return p.Jasa + p.Denda + p.BiayaAdmin }

// Nama menghasilkan teks seperti "Januari 2027".
func (p PendapatanBulan) Nama() string {
	return fmt.Sprintf("%s %d", namaBulan[p.Bulan.Month()-1], p.Bulan.Year())
}

// HitungPendapatan menjumlahkan jasa, denda, dan biaya administrasi per bulan
// berdasarkan tanggal bayar (basis kas). Masukan harus sudah membuang transaksi
// yang dibatalkan. Hasil terurut dari bulan paling awal.
func HitungPendapatan(pembayaran []model.PembayaranLaporan, alokasi []model.AlokasiLaporan) []PendapatanBulan {
	agregat := map[time.Time]*PendapatanBulan{}
	ambil := func(t time.Time) *PendapatanBulan {
		kunci := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		if p, ok := agregat[kunci]; ok {
			return p
		}

		p := &PendapatanBulan{Bulan: kunci}
		agregat[kunci] = p
		return p
	}

	for _, a := range alokasi {
		p := ambil(a.TanggalBayar)
		p.Jasa += a.Jasa
		p.Denda += a.Denda
	}

	for _, b := range pembayaran {
		ambil(b.TanggalBayar).BiayaAdmin += b.BiayaAdmin
	}

	hasil := make([]PendapatanBulan, 0, len(agregat))
	for _, p := range agregat {
		hasil = append(hasil, *p)
	}

	slices.SortFunc(hasil, func(x, y PendapatanBulan) int { return x.Bulan.Compare(y.Bulan) })
	return hasil
}

// JumlahPendapatan menjumlahkan seluruh bulan menjadi satu baris total.
func JumlahPendapatan(bulan []PendapatanBulan) PendapatanBulan {
	var t PendapatanBulan
	for _, b := range bulan {
		t.Jasa += b.Jasa
		t.Denda += b.Denda
		t.BiayaAdmin += b.BiayaAdmin
	}
	return t
}

// PendapatanInput adalah isian filter; keduanya berformat YYYY-MM dan boleh kosong.
type PendapatanInput struct {
	Dari   string
	Sampai string
}

type LaporanPendapatan struct {
	Dari   string
	Sampai string
	Bulan  []PendapatanBulan
	Total  PendapatanBulan
}

// Pendapatan menyusun laporan pendapatan per bulan. Filter kosong berarti seluruh periode.
func (s *LaporanService) Pendapatan(ctx context.Context, in PendapatanInput) (LaporanPendapatan, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return LaporanPendapatan{}, ErrBelumLogin
	}

	in.Dari, in.Sampai = strings.TrimSpace(in.Dari), strings.TrimSpace(in.Sampai)
	galat := map[string]string{}
	var dari, sampai *time.Time

	if in.Dari != "" {
		t, err := time.Parse("2006-01", in.Dari)
		if err != nil {
			galat["dari"] = "Bulan awal harus berformat tahun-bulan."
		} else {
			dari = &t
		}
	}

	if in.Sampai != "" {
		t, err := time.Parse("2006-01", in.Sampai)
		if err != nil {
			galat["sampai"] = "Bulan akhir harus berformat tahun-bulan."
		} else {
			akhir := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC) // hari terakhir bulan itu
			sampai = &akhir
		}
	}

	if dari != nil && sampai != nil && dari.After(*sampai) {
		galat["sampai"] = "Bulan akhir tidak boleh sebelum bulan awal."
	}

	if len(galat) > 0 {
		return LaporanPendapatan{}, &ValidasiError{Pesan: galat}
	}

	pembayaran, err := s.repo.Pembayaran(ctx, s.db, dari, sampai)
	if err != nil {
		return LaporanPendapatan{}, fmt.Errorf("laporan pendapatan: %w", err)
	}

	alokasi, err := s.repo.Alokasi(ctx, s.db, dari, sampai)
	if err != nil {
		return LaporanPendapatan{}, fmt.Errorf("laporan pendapatan: %w", err)
	}

	bulan := HitungPendapatan(pembayaran, alokasi)
	return LaporanPendapatan{Dari: in.Dari, Sampai: in.Sampai, Bulan: bulan, Total: JumlahPendapatan(bulan)}, nil
}

// =====================================================================
// L-2 Daftar tunggakan
// =====================================================================

// BarisTunggakan adalah satu angsuran yang lewat jatuh tempo dan belum lunas.
type BarisTunggakan struct {
	IDAnggota     int64
	NomorAnggota  string
	NamaAnggota   string
	IDPinjaman    int64
	Urutan        int
	JatuhTempo    time.Time
	HariTerlambat int
	SisaJasa      int64
	SisaPokok     int64
	DendaBerjalan int64
}

// Tagihan adalah sisa jasa ditambah sisa pokok (tanpa denda).
func (b BarisTunggakan) Tagihan() int64 { return b.SisaJasa + b.SisaPokok }

// Total adalah tagihan ditambah denda berjalan.
func (b BarisTunggakan) Total() int64 { return b.Tagihan() + b.DendaBerjalan }

// HitungTunggakan mencari angsuran yang lewat jatuh tempo pada asOf dan belum
// lunas. Status dan denda memakai helper.HitungStatus (bukan rumus tersendiri),
// sehingga angkanya sama dengan modul pembayaran. Pinjaman yang datanya tidak
// konsisten (replay gagal) tidak membatalkan laporan: namanya dikembalikan di
// daftar kedua supaya terlihat, bukan disembunyikan.
func HitungTunggakan(pinjaman []model.PinjamanLaporan, angsuran []model.Angsuran, bayar []model.Pembayaran, asOf time.Time, biayaAdminBps int) ([]BarisTunggakan, []string) {
	jadwal := map[int64][]model.Angsuran{}
	for _, a := range angsuran {
		jadwal[a.IDPinjaman] = append(jadwal[a.IDPinjaman], a)
	}

	pembayaran := map[int64][]model.Pembayaran{}
	for _, p := range bayar {
		pembayaran[p.IDPinjaman] = append(pembayaran[p.IDPinjaman], p)
	}

	hari := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	var baris []BarisTunggakan
	var dilewati []string

	for _, pj := range pinjaman {
		j := jadwal[pj.ID]
		slices.SortFunc(j, func(x, y model.Angsuran) int { return cmp.Compare(x.Urutan, y.Urutan) })

		status, err := helper.HitungStatus(j, pembayaran[pj.ID], hari, helper.AturanHitung{DendaPerHari: pj.DendaPerHari, BiayaAdminBps: biayaAdminBps})
		if err != nil {
			dilewati = append(dilewati, fmt.Sprintf("Pinjaman #%d (%s): %v", pj.ID, pj.NomorAnggota, err))
			continue
		}

		for _, st := range status {
			jt := time.Date(st.JatuhTempo.Year(), st.JatuhTempo.Month(), st.JatuhTempo.Day(), 0, 0, 0, 0, time.UTC)
			if !jt.Before(hari) || st.IsLunas() { // tanggal jatuh tempo itu sendiri belum terlambat
				continue
			}

			baris = append(baris, BarisTunggakan{
				IDAnggota:     pj.IDAnggota,
				NomorAnggota:  pj.NomorAnggota,
				NamaAnggota:   pj.NamaAnggota,
				IDPinjaman:    pj.ID,
				Urutan:        st.Urutan,
				JatuhTempo:    jt,
				HariTerlambat: int(hari.Sub(jt) / (24 * time.Hour)),
				SisaJasa:      st.SisaJasa(),
				SisaPokok:     st.SisaPokok(),
				DendaBerjalan: st.DendaTerutang,
			})
		}
	}

	// paling lama menunggak di atas
	slices.SortStableFunc(baris, func(x, y BarisTunggakan) int {
		if c := cmp.Compare(y.HariTerlambat, x.HariTerlambat); c != 0 {
			return c
		}
		if c := strings.Compare(x.NomorAnggota, y.NomorAnggota); c != 0 {
			return c
		}
		return cmp.Compare(x.Urutan, y.Urutan)
	})

	return baris, dilewati
}

// TunggakanInput: Tanggal berformat YYYY-MM-DD, kosong berarti hari ini.
type TunggakanInput struct {
	Tanggal string
}

type LaporanTunggakan struct {
	Tanggal       time.Time
	Baris         []BarisTunggakan
	JumlahAnggota int
	TotalTagihan  int64
	TotalDenda    int64
	Dilewati      []string
}

// TotalSemua adalah total tagihan ditambah total denda.
func (l LaporanTunggakan) TotalSemua() int64 { return l.TotalTagihan + l.TotalDenda }

// Tunggakan menyusun daftar tunggakan sampai tanggal yang dipilih.
func (s *LaporanService) Tunggakan(ctx context.Context, in TunggakanInput) (LaporanTunggakan, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return LaporanTunggakan{}, ErrBelumLogin
	}

	asOf := s.hariIni()
	if t := strings.TrimSpace(in.Tanggal); t != "" {
		d, err := time.Parse("2006-01-02", t)
		if err != nil {
			return LaporanTunggakan{}, &ValidasiError{Pesan: map[string]string{"tanggal": "Tanggal harus berformat tahun-bulan-hari."}}
		}
		asOf = d
	}

	pinjaman, err := s.repo.PinjamanBerjalan(ctx, s.db, asOf)
	if err != nil {
		return LaporanTunggakan{}, fmt.Errorf("laporan tunggakan: %w", err)
	}

	angsuran, err := s.repo.AngsuranBerjalan(ctx, s.db, asOf)
	if err != nil {
		return LaporanTunggakan{}, fmt.Errorf("laporan tunggakan: %w", err)
	}

	bayar, err := s.repo.PembayaranBerjalan(ctx, s.db, asOf)
	if err != nil {
		return LaporanTunggakan{}, fmt.Errorf("laporan tunggakan: %w", err)
	}

	baris, dilewati := HitungTunggakan(pinjaman, angsuran, bayar, asOf, s.biayaAdminBps)
	hasil := LaporanTunggakan{Tanggal: asOf, Baris: baris, Dilewati: dilewati}
	anggota := map[int64]bool{}
	for _, b := range baris {
		anggota[b.IDAnggota] = true
		hasil.TotalTagihan += b.Tagihan()
		hasil.TotalDenda += b.DendaBerjalan
	}
	hasil.JumlahAnggota = len(anggota)

	return hasil, nil
}
