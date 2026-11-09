package helper

import (
	"errors"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

const TanggalJatuhTempo = 10

// kesalahan validasi masukan jadwal
var (
	ErrJumlahTidakValid = errors.New("helper: jumlah pinjaman harus lebh dari 0")
	ErrTenorTidakValid  = errors.New("helper: tenor harus lebih dari 0")
	ErrJasaTidakValid   = errors.New("helper: jasa tidak boleh negatif")
)

// membuat jadwal angsuran unuk peminjaman
// batas tenor diperiksa oleh use case memakai konfigurasi, bukan disini
func BuatJadwal(jumlah int64, tenor, jasaBps int, pencairan time.Time) ([]model.Angsuran, error) {
	switch {
	case jumlah <= 0:
		return nil, ErrJumlahTidakValid
	case tenor <= 0:
		return nil, ErrTenorTidakValid
	case jasaBps < 0:
		return nil, ErrJasaTidakValid
	}

	n := int64(tenor)
	jasaTotal := jumlah * int64(jasaBps) * n / 10000
	pokokTiap := jumlah / n
	jasaTiap := jasaTotal / n

	jadwal := make([]model.Angsuran, 0, tenor)
	for i := 1; i <= tenor; i++ {
		jasa, pokok := jasaTiap, pokokTiap
		if i == tenor {
			pokok = jumlah - pokokTiap*(n-1)
			jasa = jasaTotal - jasaTiap*(n-1)
		}

		jadwal = append(jadwal, model.Angsuran{
			Urutan:     i,
			JatuhTempo: jatuhTempoKe(pencairan, i),
			Pokok:      pokok,
			Jasa:       jasa,
		})
	}

	return jadwal, nil
}

func jatuhTempoKe(pencairan time.Time, k int) time.Time {
	return time.Date(pencairan.Year(), pencairan.Month()+time.Month(k), TanggalJatuhTempo, 0, 0, 0, 0, pencairan.Location())
}
