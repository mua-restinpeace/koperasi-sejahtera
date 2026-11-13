package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type AnggotaService interface {
	Register(ctx context.Context, in service.DaftarAnggotaInput) (int64, error)
	FormatTanggal() time.Time
}

type dataListAnggota struct {
	Anggota      []model.Anggota
	Cari         string
	FilterStatus string

	Total  int
	Mulai  int
	Sampai int

	Halaman
	CurrentHalaman int
	HalamanPrev    int
	HalamanNext    int
	TotalHalaman   int
	NomorHalaman   []int
}

type dataFormAnggota struct {
	Halaman
	Isian service.DaftarAnggotaInput
	Error map[string]string
}

// ListAnggota menampilkan seluruh anggota
func (h *Handler) ListAnggota(w http.ResponseWriter, r *http.Request) {
	hal := h.halaman(r, "Anggota", "anggota")
	if r.URL.Query().Get("berhasil") == "1" {
		hal.Pesan = "Anggota berhasil didaftarkan"
	}
	h.render(w, http.StatusOK, "pages/anggota.html", dataListAnggota{
		Halaman: hal,
		Anggota: nil,
	})
}

// FormAnggota menampilkan form pendaftaran anggota baru
func (h *Handler) FormAnggota(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "pages/anggota_form.html", dataFormAnggota{
		Halaman: h.halaman(r, "Daftar Anggota Baru", "anggota"),
		Isian: service.DaftarAnggotaInput{
			TanggalBergabung: h.svc.Anggota.FormatTanggal().Format("2006-01-02"),
		},
	})
}

// SimpanAnggota memproses form pendaftaran.
// Bila validasi gagal, form ditampilkan ulang dengan isian dan pesan kesalahan (status 422).
func (h *Handler) SimpanAnggota(w http.ResponseWriter, r *http.Request) {
	in := service.DaftarAnggotaInput{
		NomorAnggota:     r.PostFormValue("nomor_anggota"),
		NamaLengkap:      r.PostFormValue("nama_lengkap"),
		NoTelepon:        r.PostFormValue("no_telepon"),
		TanggalBergabung: r.PostFormValue("tanggal_bergabung"),
	}

	_, err := h.svc.Anggota.Register(r.Context(), in)
	var ve *service.ValidasiError
	switch {
	case errors.As(err, &ve):
		h.render(w, http.StatusUnprocessableEntity, "pages/anggota_form.html", dataFormAnggota{
			Halaman: h.halaman(r, "Daftar Anggota Baru", "anggota"),
			Isian:   in,
			Error:   ve.Pesan,
		})
	case err != nil:
		h.error(w, r, err)
	default:
		http.Redirect(w, r, "/anggota?berhasil=1", http.StatusSeeOther)
	}
}
