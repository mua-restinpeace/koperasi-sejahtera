package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

// ErrPinjamanTidakDitemukan dikembalikan kartu pinjaman untuk nomor pinjaman yang
// tidak ada atau sudah dibatalkan.
var ErrPinjamanTidakDitemukan = errors.New("pinjaman tidak ditemukan")

// =====================================================================
// L-1 Kartu pinjaman
// =====================================================================

// BarisKartu adalah keadaan satu angsuran pada tanggal kartu.
type BarisKartu struct {
	Urutan        int
	JatuhTempo    time.Time
	TagihanPokok  int64
	TagihanJasa   int64
	PokokDibayar  int64
	JasaDibayar   int64
	JasaDihapus   int64
	DendaDibayar  int64
	SisaPokok     int64
	SisaJasa      int64
	DendaTerutang int64
	LunasPada     time.Time // nol bila belum lunas
	HariTerlambat int       // hanya untuk angsuran yang belum lunas dan lewat jatuh tempo
	Status        string
}

// Kelas memberi nama kelas lencana (badge) untuk status angsuran.
func (b BarisKartu) Kelas() string {
	switch b.Status {
	case StatusKartuLunas:
		return "badge-ok"
	case StatusKartuMenunggak:
		return "badge-er"
	case StatusKartuSebagian:
		return "badge-wa"
	}
	return "badge-gr"
}

// Tagihan adalah pokok ditambah jasa yang ditagihkan pada angsuran ini.
func (b BarisKartu) Tagihan() int64 { return b.TagihanPokok + b.TagihanJasa }

// Dibayar adalah pokok ditambah jasa yang sudah dibayar (tanpa denda).
func (b BarisKartu) Dibayar() int64 { return b.PokokDibayar + b.JasaDibayar }

// SisaTagihan adalah sisa jasa ditambah sisa pokok (tanpa denda).
func (b BarisKartu) SisaTagihan() int64 { return b.SisaPokok + b.SisaJasa }

const (
	StatusKartuLunas     = "Lunas"
	StatusKartuMenunggak = "Menunggak"
	StatusKartuSebagian  = "Dibayar sebagian"
	StatusKartuBelum     = "Belum jatuh tempo"
)

// KartuPinjaman adalah kartu satu pinjaman: induk, jadwal dengan status, dan
// riwayat pembayaran sampai tanggal kartu.
type KartuPinjaman struct {
	Pinjaman   model.PinjamanKartu
	Tanggal    time.Time
	Baris      []BarisKartu
	Pembayaran []model.Pembayaran

	TotalDibayar  int64 // pokok + jasa + denda yang sudah masuk dari angsuran
	SisaPokok     int64
	SisaJasa      int64
	DendaTerutang int64
}

// SisaJasaDenda adalah sisa jasa ditambah denda terutang.
func (k KartuPinjaman) SisaJasaDenda() int64 { return k.SisaJasa + k.DendaTerutang }

// SisaTagihan adalah sisa pokok + sisa jasa + denda terutang.
func (k KartuPinjaman) SisaTagihan() int64 { return k.SisaPokok + k.SisaJasa + k.DendaTerutang }

// HitungKartu menyusun kartu dari data mentah. Status dan denda memakai
// helper.HitungStatus, sama seperti modul pembayaran dan daftar tunggakan.
func HitungKartu(pj model.PinjamanKartu, jadwal []model.Angsuran, bayar []model.Pembayaran, asOf time.Time, biayaAdminBps int) (KartuPinjaman, error) {
	hari := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)

	status, err := helper.HitungStatus(jadwal, bayar, hari, helper.AturanHitung{DendaPerHari: pj.DendaPerHari, BiayaAdminBps: biayaAdminBps})
	if err != nil {
		return KartuPinjaman{}, fmt.Errorf("kartu pinjaman #%d: %w", pj.ID, err)
	}

	k := KartuPinjaman{Pinjaman: pj, Tanggal: hari}
	for _, p := range bayar {
		if !p.TanggalBayar.After(hari) {
			k.Pembayaran = append(k.Pembayaran, p)
		}
	}

	for _, st := range status {
		jt := time.Date(st.JatuhTempo.Year(), st.JatuhTempo.Month(), st.JatuhTempo.Day(), 0, 0, 0, 0, time.UTC)
		b := BarisKartu{
			Urutan:        st.Urutan,
			JatuhTempo:    jt,
			TagihanPokok:  st.Pokok,
			TagihanJasa:   st.Jasa,
			PokokDibayar:  st.PokokDibayar,
			JasaDibayar:   st.JasaDibayar,
			JasaDihapus:   st.JasaDihapus,
			DendaDibayar:  st.DendaDibayar,
			SisaPokok:     st.SisaPokok(),
			SisaJasa:      st.SisaJasa(),
			DendaTerutang: st.DendaTerutang,
			LunasPada:     st.LunasPada,
		}

		switch {
		case st.IsLunas():
			b.Status = StatusKartuLunas
		case jt.Before(hari):
			b.Status = StatusKartuMenunggak
			b.HariTerlambat = int(hari.Sub(jt) / (24 * time.Hour))
		case st.PokokDibayar+st.JasaDibayar > 0:
			b.Status = StatusKartuSebagian
		default:
			b.Status = StatusKartuBelum
		}

		k.Baris = append(k.Baris, b)
		k.TotalDibayar += b.PokokDibayar + b.JasaDibayar + b.DendaDibayar
		k.SisaPokok += b.SisaPokok
		k.SisaJasa += b.SisaJasa
		k.DendaTerutang += b.DendaTerutang
	}

	return k, nil
}

// DaftarPinjaman mengembalikan semua pinjaman yang tidak dibatalkan, terbaru di atas.
func (s *LaporanService) DaftarPinjaman(ctx context.Context) ([]model.PinjamanKartu, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return nil, ErrBelumLogin
	}

	hasil, err := s.repo.DaftarPinjaman(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("daftar pinjaman: %w", err)
	}
	return hasil, nil
}

// KartuInput: ID pinjaman, dan Tanggal berformat YYYY-MM-DD (kosong = hari ini).
type KartuInput struct {
	ID      int64
	Tanggal string
}

// KartuPinjaman menyusun kartu pinjaman pada tanggal yang dipilih.
func (s *LaporanService) KartuPinjaman(ctx context.Context, in KartuInput) (KartuPinjaman, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return KartuPinjaman{}, ErrBelumLogin
	}

	asOf := s.hariIni()
	if t := strings.TrimSpace(in.Tanggal); t != "" {
		d, err := time.Parse("2006-01-02", t)
		if err != nil {
			return KartuPinjaman{}, &ValidasiError{Pesan: map[string]string{"tanggal": "Tanggal harus berformat tahun-bulan-hari."}}
		}
		asOf = d
	}

	pj, err := s.repo.DetailPinjaman(ctx, s.db, in.ID)
	if errors.Is(err, repository.ErrTidakDitemukan) {
		return KartuPinjaman{}, ErrPinjamanTidakDitemukan
	}
	if err != nil {
		return KartuPinjaman{}, fmt.Errorf("kartu pinjaman: %w", err)
	}

	jadwal, err := s.repo.AngsuranPinjaman(ctx, s.db, in.ID)
	if err != nil {
		return KartuPinjaman{}, fmt.Errorf("kartu pinjaman: %w", err)
	}

	bayar, err := s.repo.PembayaranPinjaman(ctx, s.db, in.ID)
	if err != nil {
		return KartuPinjaman{}, fmt.Errorf("kartu pinjaman: %w", err)
	}

	return HitungKartu(pj, jadwal, bayar, asOf, s.biayaAdminBps)
}
