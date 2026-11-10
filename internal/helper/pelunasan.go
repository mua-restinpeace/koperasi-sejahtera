package helper

import (
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

// menghitung jumlah pelunasan dipercepat pada tanggal asOf
func HitungPelunasan(status []model.StatusAngsuran, asOf time.Time, biayaAdminBps int) model.RincianPelunasan {
	d := tanggalSaja(asOf)

	var r model.RincianPelunasan
	for _, s := range status {
		r.SisaPokok += s.SisaPokok()
		r.Denda += s.DendaTerutang

		if sudahJatuhTempo(s, d) {
			r.JasaJatuhTempo += s.SisaJasa()
		}
	}

	if i := indeksBulanBerjalan(status, d); i >= 0 {
		r.JasaBulanBerjalan = status[i].SisaJasa()
	}

	r.BiayaAdmin = r.SisaPokok * int64(biayaAdminBps) / 10000
	r.Total = r.SisaPokok + r.Denda + r.JasaJatuhTempo + r.JasaBulanBerjalan + r.BiayaAdmin
	return r
}
