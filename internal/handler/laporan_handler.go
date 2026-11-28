package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type LaporanService interface {
	Pendapatan(ctx context.Context, in service.PendapatanInput) (service.LaporanPendapatan, error)
	Tunggakan(ctx context.Context, in service.TunggakanInput) (service.LaporanTunggakan, error)
}

type dataPendapatan struct {
	Halaman
	service.LaporanPendapatan
	Error map[string]string
}

type dataTunggakan struct {
	Halaman
	service.LaporanTunggakan
	TanggalInput string // isi kolom tanggal (YYYY-MM-DD)
	Error        map[string]string
}

// Laporan menampilkan daftar laporan yang tersedia.
func (h *Handler) Laporan(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "pages/laporan.html", h.halaman(r, "Laporan", "laporan"))
}

// LaporanPendapatan menampilkan pendapatan jasa, denda, dan biaya administrasi per bulan (L-3).
// Filter: ?dari=YYYY-MM&sampai=YYYY-MM (kosong = seluruh periode).
func (h *Handler) LaporanPendapatan(w http.ResponseWriter, r *http.Request) {
	in := service.PendapatanInput{Dari: r.URL.Query().Get("dari"), Sampai: r.URL.Query().Get("sampai")}
	hal := h.halaman(r, "Pendapatan Bulanan", "laporan")

	hasil, err := h.svc.Laporan.Pendapatan(r.Context(), in)
	var ve *service.ValidasiError
	switch {
	case errors.As(err, &ve):
		hasil = service.LaporanPendapatan{Dari: in.Dari, Sampai: in.Sampai}
		h.render(w, http.StatusUnprocessableEntity, "pages/laporan_pendapatan.html", dataPendapatan{Halaman: hal, LaporanPendapatan: hasil, Error: ve.Pesan})
	case err != nil:
		h.error(w, r, err)
	default:
		h.render(w, http.StatusOK, "pages/laporan_pendapatan.html", dataPendapatan{Halaman: hal, LaporanPendapatan: hasil})
	}
}

// LaporanTunggakan menampilkan angsuran yang lewat jatuh tempo dan belum lunas (L-2).
// Filter: ?tanggal=YYYY-MM-DD (kosong = hari ini).
func (h *Handler) LaporanTunggakan(w http.ResponseWriter, r *http.Request) {
	in := service.TunggakanInput{Tanggal: r.URL.Query().Get("tanggal")}
	hal := h.halaman(r, "Daftar Tunggakan", "laporan")

	hasil, err := h.svc.Laporan.Tunggakan(r.Context(), in)
	var ve *service.ValidasiError
	switch {
	case errors.As(err, &ve):
		h.render(w, http.StatusUnprocessableEntity, "pages/laporan_tunggakan.html", dataTunggakan{Halaman: hal, TanggalInput: in.Tanggal, Error: ve.Pesan})
	case err != nil:
		h.error(w, r, err)
	default:
		h.render(w, http.StatusOK, "pages/laporan_tunggakan.html", dataTunggakan{Halaman: hal, LaporanTunggakan: hasil, TanggalInput: hasil.Tanggal.Format("2006-01-02")})
	}
}
