package helper

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

var (
	ErrPembayaranBerlebih         = errors.New("helper: pembayaran melebihi seluruh sisa kewajiban")
	ErrPinjamanSudahLunas         = errors.New("helper: pinjaman sudah lunas")
	ErrJumlahPembayaranTidakValid = errors.New("helper: jumlah pembayaran harus lebih dari 0")
	ErrBiayaAdminTidakValid       = errors.New("helper: biaya administrasi hanya untuk pelunasan")
	ErrJumlahPelunasanTidakSesuai = errors.New("helper: jumlah pelunasan tidak sesuai perhitungan")
	ErrJenisTidakDikenal          = errors.New("helper: jenis pembayaran tidak dikenal")
)

// parameter aturan koperasi
type AturanHitung struct {
	DendaPerHari  int64
	BiayaAdminBps int
}

// menghitung pembagian setiap pembayaran ke denda, jasa, dan pokok
func HitungAlokasi(jadwal []model.Angsuran, pembayaran []model.Pembayaran, aturan AturanHitung) ([]model.Alokasi, error) {
	_, alokasi, err := replay(jadwal, pembayaran, aturan)
	return alokasi, err
}

// menghitung keadaan setiap angsuran pada tanggal asOf
func HitungStatus(jadwal []model.Angsuran, pembayaran []model.Pembayaran, asOf time.Time, aturan AturanHitung) ([]model.StatusAngsuran, error) {
	d := tanggalSaja(asOf)
	var sampai []model.Pembayaran

	for _, p := range pembayaran {
		if !tanggalSaja(p.TanggalBayar).After(d) {
			sampai = append(sampai, p)
		}
	}

	status, _, err := replay(jadwal, sampai, aturan)
	if err != nil {
		return nil, err
	}

	return denganDenda(status, d, aturan), nil
}

// // menerapkan pembayaran dari yang paling awal ke jadwal
func replay(jadwal []model.Angsuran, pembayaran []model.Pembayaran, aturan AturanHitung) ([]model.StatusAngsuran, []model.Alokasi, error) {
	status := make([]model.StatusAngsuran, len(jadwal))
	for i, a := range jadwal {
		status[i] = model.StatusAngsuran{Angsuran: a}
	}

	urut := slices.Clone(pembayaran)
	slices.SortStableFunc(urut, func(x, y model.Pembayaran) int {
		if c := tanggalSaja(x.TanggalBayar).Compare(tanggalSaja(y.TanggalBayar)); c != 0 {
			return c
		}

		return cmp.Compare(x.ID, y.ID)
	})

	var alokasi []model.Alokasi
	for _, p := range urut {
		if IsSemuaLunas(status) {
			return nil, nil, fmt.Errorf("pembayaran %d: %w", p.ID, ErrPinjamanSudahLunas)
		}

		if p.Jumlah <= 0 {
			return nil, nil, fmt.Errorf("pembayaran %d: %w", p.ID, ErrJumlahPembayaranTidakValid)
		}

		d := tanggalSaja(p.TanggalBayar)
		var baris []model.Alokasi
		var err error
		switch p.Jenis {
		case model.PembayaranAngsuran:
			baris, err = terapkanAngsuran(status, p, d, aturan)
		case model.PembayaranPelunasan:
			baris, err = terapkanPelunasan(status, p, d, aturan)
		default:
			err = fmt.Errorf("%w: %q", ErrJenisTidakDikenal, p.Jenis)
		}

		if err != nil {
			return nil, nil, fmt.Errorf("pembayaran %d: %w", p.ID, err)
		}

		alokasi = append(alokasi, baris...)
	}

	return status, alokasi, nil

}

func terapkanAngsuran(status []model.StatusAngsuran, p model.Pembayaran, d time.Time, aturan AturanHitung) ([]model.Alokasi, error) {
	if p.BiayaAdmin != 0 {
		return nil, ErrBiayaAdminTidakValid
	}

	sisa := p.Jumlah
	var baris []model.Alokasi
	for i := range status {
		if sisa == 0 {
			break
		}

		s := &status[i]
		if s.IsLunas() {
			continue
		}

		a := model.Alokasi{IDPembayaran: p.ID, Urutan: s.Urutan}
		a.Denda = min(sisa, dendaTerutang(*s, d, aturan))
		sisa -= a.Denda
		a.Jasa = min(sisa, s.SisaJasa())
		sisa -= a.Jasa
		a.Pokok = min(sisa, s.SisaPokok())
		sisa -= a.Pokok

		catat(s, a, d)
		if a.Denda+a.Jasa+a.Pokok > 0 {
			baris = append(baris, a)
		}
	}

	if sisa > 0 {
		return nil, fmt.Errorf("%w, kelebihan Rp %d", ErrPembayaranBerlebih, sisa)
	}

	return baris, nil
}

func terapkanPelunasan(status []model.StatusAngsuran, p model.Pembayaran, d time.Time, aturan AturanHitung) ([]model.Alokasi, error) {
	sd := denganDenda(status, d, aturan)
	r := HitungPelunasan(sd, d, aturan.BiayaAdminBps)
	if p.Jumlah != r.Total || p.BiayaAdmin != r.BiayaAdmin {
		return nil, fmt.Errorf("%w: seharusnya Rp %d (administrasi Rp %d), diterima Rp %d (administrasi Rp %d)", ErrJumlahPelunasanTidakSesuai, r.Total, r.BiayaAdmin, p.Jumlah, p.BiayaAdmin)
	}

	berjalan := indeksBulanBerjalan(status, d)
	var baris []model.Alokasi
	for i := range status {
		s := &status[i]
		if s.IsLunas() {
			continue
		}

		a := model.Alokasi{
			IDPembayaran: p.ID,
			Urutan:       s.Urutan,
			Denda:        sd[i].DendaTerutang,
			Pokok:        s.SisaPokok(),
		}

		if sudahJatuhTempo(*s, d) || i == berjalan {
			a.Jasa = s.SisaJasa()
		}

		catat(s, a, d)
		s.JasaDihapus += s.SisaJasa() // jasa bulan berikutnya tidak ditagihkan
		if s.LunasPada.IsZero() {
			s.LunasPada = d
		}

		if a.Denda+a.Jasa+a.Pokok > 0 {
			baris = append(baris, a)
		}
	}

	return baris, nil
}

// menambahkan alokasi ke angsuran dan menandai tanggal lunasnya
func catat(s *model.StatusAngsuran, a model.Alokasi, d time.Time) {
	s.DendaDibayar += a.Denda
	s.JasaDibayar += a.Jasa
	s.PokokDibayar += a.Pokok
	if s.IsLunas() && s.LunasPada.IsZero() {
		s.LunasPada = d
	}
}

func dendaTerutang(s model.StatusAngsuran, d time.Time, aturan AturanHitung) int64 {
	akhir := d
	if s.IsLunas() {
		akhir = s.LunasPada
	}

	return max(HitungDenda(s.JatuhTempo, akhir, aturan.DendaPerHari)-s.DendaDibayar, 0)
}

func denganDenda(status []model.StatusAngsuran, d time.Time, aturan AturanHitung) []model.StatusAngsuran {
	hasil := slices.Clone(status)
	for i := range hasil {
		hasil[i].DendaTerutang = dendaTerutang(hasil[i], d, aturan)
	}

	return hasil
}

func sudahJatuhTempo(s model.StatusAngsuran, d time.Time) bool {
	return !tanggalSaja(s.JatuhTempo).After(d)
}

func indeksBulanBerjalan(status []model.StatusAngsuran, d time.Time) int {
	for i, s := range status {
		if !sudahJatuhTempo(s, d) && s.SisaJasa() > 0 {
			return i
		}
	}
	return -1
}

func IsSemuaLunas(status []model.StatusAngsuran) bool {
	for _, s := range status {
		if !s.IsLunas() {
			return false
		}
	}
	return true
}
