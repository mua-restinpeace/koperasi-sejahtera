package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type LaporanService interface {
	Pendapatan(ctx context.Context, in service.PendapatanInput) (service.LaporanPendapatan, error)
	Tunggakan(ctx context.Context, in service.TunggakanInput) (service.LaporanTunggakan, error)
	DaftarPinjaman(ctx context.Context) ([]model.PinjamanKartu, error)
	KartuPinjaman(ctx context.Context, in service.KartuInput) (service.KartuPinjaman, error)
	Dashboard(ctx context.Context) (service.Dashboard, error)
}

type dataDaftarPinjaman struct {
	Halaman
	Pinjaman []model.PinjamanKartu
}

type dataKartu struct {
	Halaman
	service.KartuPinjaman
	TanggalInput string // isi kolom tanggal (YYYY-MM-DD)
	Error        map[string]string
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

// LaporanPinjaman menampilkan daftar pinjaman; tiap baris mengarah ke kartu pinjamannya (L-1).
func (h *Handler) LaporanPinjaman(w http.ResponseWriter, r *http.Request) {
	daftar, err := h.svc.Laporan.DaftarPinjaman(r.Context())
	if err != nil {
		h.error(w, r, err)
		return
	}
	h.render(w, http.StatusOK, "pages/laporan_pinjaman.html", dataDaftarPinjaman{Halaman: h.halaman(r, "Kartu Pinjaman", "laporan"), Pinjaman: daftar})
}

// LaporanKartu menampilkan kartu satu pinjaman: jadwal angsuran dengan status dan
// riwayat pembayaran (L-1). Filter: ?tanggal=YYYY-MM-DD (kosong = hari ini).
func (h *Handler) LaporanKartu(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	in := service.KartuInput{ID: id, Tanggal: r.URL.Query().Get("tanggal")}
	hal := h.halaman(r, "Kartu Pinjaman", "laporan")

	hasil, err := h.svc.Laporan.KartuPinjaman(r.Context(), in)
	var ve *service.ValidasiError
	switch {
	case errors.Is(err, service.ErrPinjamanTidakDitemukan):
		h.notFound(w)
	case errors.As(err, &ve):
		h.render(w, http.StatusUnprocessableEntity, "pages/laporan_kartu.html", dataKartu{Halaman: hal, TanggalInput: in.Tanggal, Error: ve.Pesan, KartuPinjaman: service.KartuPinjaman{Pinjaman: model.PinjamanKartu{ID: id}}})
	case err != nil:
		h.error(w, r, err)
	default:
		h.render(w, http.StatusOK, "pages/laporan_kartu.html", dataKartu{Halaman: hal, KartuPinjaman: hasil, TanggalInput: hasil.Tanggal.Format("2006-01-02")})
	}
}

// kirimCSV mengirim berkas CSV sebagai unduhan.
func (h *Handler) kirimCSV(w http.ResponseWriter, r *http.Request, nama string, isi []byte, err error) {
	if err != nil {
		h.error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+nama+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(isi)
}

// LaporanPendapatanCSV mengunduh L-3 sebagai CSV; filter sama dengan halamannya.
func (h *Handler) LaporanPendapatanCSV(w http.ResponseWriter, r *http.Request) {
	hasil, err := h.svc.Laporan.Pendapatan(r.Context(), service.PendapatanInput{Dari: r.URL.Query().Get("dari"), Sampai: r.URL.Query().Get("sampai")})
	var ve *service.ValidasiError
	if errors.As(err, &ve) {
		http.Error(w, "Filter tidak valid", http.StatusBadRequest)
		return
	}
	if err != nil {
		h.error(w, r, err)
		return
	}
	isi, errCSV := hasil.CSV()
	h.kirimCSV(w, r, "pendapatan-bulanan.csv", isi, errCSV)
}

// LaporanTunggakanCSV mengunduh L-2 sebagai CSV; filter sama dengan halamannya.
func (h *Handler) LaporanTunggakanCSV(w http.ResponseWriter, r *http.Request) {
	hasil, err := h.svc.Laporan.Tunggakan(r.Context(), service.TunggakanInput{Tanggal: r.URL.Query().Get("tanggal")})
	var ve *service.ValidasiError
	if errors.As(err, &ve) {
		http.Error(w, "Tanggal tidak valid", http.StatusBadRequest)
		return
	}
	if err != nil {
		h.error(w, r, err)
		return
	}
	isi, errCSV := hasil.CSV()
	h.kirimCSV(w, r, "tunggakan-"+hasil.Tanggal.Format("2006-01-02")+".csv", isi, errCSV)
}

// LaporanKartuCSV mengunduh kartu pinjaman (L-1) sebagai CSV.
func (h *Handler) LaporanKartuCSV(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	hasil, err := h.svc.Laporan.KartuPinjaman(r.Context(), service.KartuInput{ID: id, Tanggal: r.URL.Query().Get("tanggal")})
	var ve *service.ValidasiError
	switch {
	case errors.Is(err, service.ErrPinjamanTidakDitemukan):
		h.notFound(w)
		return
	case errors.As(err, &ve):
		http.Error(w, "Tanggal tidak valid", http.StatusBadRequest)
		return
	case err != nil:
		h.error(w, r, err)
		return
	}
	isi, errCSV := hasil.CSV()
	h.kirimCSV(w, r, "kartu-pinjaman-"+strconv.FormatInt(id, 10)+".csv", isi, errCSV)
}
