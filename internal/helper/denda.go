package helper

import "time"

// menghitung denda untuk satu angsuran
func HitungDenda(jatuhTempo, tanggalAkhir time.Time, dendaPerHari int64) int64 {
	hari := selisihHari(jatuhTempo, tanggalAkhir)
	if hari <= 0 {
		return 0
	}

	return int64(hari) * dendaPerHari
}

// membuang jam dan zona waktu
func tanggalSaja(t time.Time) time.Time {
	tahun, bulan, hari := t.Date()
	return time.Date(tahun, bulan, hari, 0, 0, 0, 0, time.UTC)
}

func selisihHari(dari, sampai time.Time) int {
	return int(tanggalSaja(sampai).Sub(tanggalSaja(dari)) / (24 * time.Hour))
}
