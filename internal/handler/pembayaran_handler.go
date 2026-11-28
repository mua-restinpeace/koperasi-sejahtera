package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type HalamanDetailPinjaman struct {
	Halaman
	Detail service.DetailPinjaman
	Error  map[string]string
}

// DetailPinjaman menampilkan informasi pinjaman dan riwayat pembayarannya.
func (h *Handler) DetailPinjaman(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	h.halamanDetailPinjaman(
		w,
		r,
		id,
		http.StatusOK,
		nil,
		pesanBerhasilPembayaran(r.URL.Query().Get("berhasil")),
	)
}

// CatatPembayaran mencatat pembayaran angsuran.
func (h *Handler) CatatPembayaran(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	tanggal, _ := time.Parse("2006-01-02", r.PostFormValue("tanggal_bayar"))

	_, err := h.svc.Pembayaran.CatatPembayaran(
		r.Context(),
		id,
		service.CatatPembayaranInput{
			TanggalBayar: tanggal,
			Jumlah:       parseJumlah(r.PostFormValue("jumlah")),
			Keterangan:   r.PostFormValue("keterangan"),
		},
	)

	h.hasilPembayaran(w, r, id, "pembayaran", err)
}

// CatatPelunasan mencatat pelunasan pinjaman.
func (h *Handler) CatatPelunasan(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	tanggal, _ := time.Parse("2006-01-02", r.PostFormValue("tanggal_bayar"))

	_, err := h.svc.Pembayaran.CatatPelunasan(
		r.Context(),
		id,
		service.CatatPelunasanInput{
			TanggalBayar: tanggal,
			Keterangan:   r.PostFormValue("keterangan"),
		},
	)

	h.hasilPembayaran(w, r, id, "pelunasan", err)
}

// BatalkanPembayaran membatalkan pembayaran berdasarkan ID pembayaran.
func (h *Handler) BatalkanPembayaran(w http.ResponseWriter, r *http.Request) {
	idPinjaman, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	idPembayaran, err := strconv.ParseInt(r.PathValue("pembayaranID"), 10, 64)
	if err != nil || idPembayaran <= 0 {
		h.notFound(w)
		return
	}

	_, err = h.svc.Pembayaran.BatalkanPembayaran(
		r.Context(),
		idPembayaran,
		r.PostFormValue("alasan"),
	)

	h.hasilPembayaran(w, r, idPinjaman, "batal", err)
}

// halamanDetailPinjaman memuat ulang halaman detail beserta pesan kesalahan.
func (h *Handler) halamanDetailPinjaman(
	w http.ResponseWriter,
	r *http.Request,
	id int64,
	status int,
	galat map[string]string,
	pesan string,
) {
	detail, err := h.svc.Pembayaran.GetDetailPinjaman(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrPinjamanTidakDitemukan) {
			h.notFound(w)
			return
		}

		h.error(w, r, err)
		return
	}

	hal := h.halaman(r, "Detail Pinjaman", "pinjaman")
	hal.Pesan = pesan

	data := HalamanDetailPinjaman{
		Halaman: hal,
		Detail:  detail,
		Error:   galat,
	}

	h.render(w, status, "pages/pinjaman_detail.html", data)
}

// hasilPembayaran menangani hasil transaksi pembayaran.
func (h *Handler) hasilPembayaran(
	w http.ResponseWriter,
	r *http.Request,
	idPinjaman int64,
	kode string,
	err error,
) {
	var ve *service.ValidasiError

	switch {
	case err == nil:
		http.Redirect(
			w,
			r,
			fmt.Sprintf("/pinjaman/%d?berhasil=%s", idPinjaman, kode),
			http.StatusSeeOther,
		)

	case errors.Is(err, service.ErrPinjamanTidakDitemukan),
		errors.Is(err, service.ErrPembayaranTidakDitemukan):
		h.notFound(w)

	case errors.As(err, &ve):
		h.halamanDetailPinjaman(
			w,
			r,
			idPinjaman,
			http.StatusUnprocessableEntity,
			ve.Pesan,
			"",
		)

	case errors.Is(err, service.ErrPinjamanTidakAktif),
		errors.Is(err, service.ErrPembayaranSudahDibatalkan):
		h.halamanDetailPinjaman(
			w,
			r,
			idPinjaman,
			http.StatusConflict,
			map[string]string{"aturan": err.Error() + "."},
			"",
		)

	default:
		h.error(w, r, err)
	}
}

// parseJumlah mengubah nilai jumlah dari formulir menjadi bilangan bulat.
// Nilai tidak valid akan menjadi 0 dan ditolak oleh validasi service.
func parseJumlah(nilai string) int64 {
	jumlah, err := strconv.ParseInt(nilai, 10, 64)
	if err != nil {
		return 0
	}

	return jumlah
}

// pesanBerhasilPembayaran menentukan pesan setelah transaksi berhasil.
func pesanBerhasilPembayaran(kode string) string {
	switch kode {
	case "pembayaran":
		return "Pembayaran angsuran berhasil dicatat."
	case "pelunasan":
		return "Pelunasan pinjaman berhasil dicatat."
	case "batal":
		return "Pembayaran berhasil dibatalkan."
	default:
		return ""
	}
}
