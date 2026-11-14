package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type AnggotaService interface {
	Register(ctx context.Context, in service.DaftarAnggotaInput) (int64, error)
	GetAll(ctx context.Context) ([]model.Anggota, error)
	GetListFiltered(ctx context.Context, cari, status string, page, limit int) (service.PaginatedAnggotaResponse, error)
	FormatTanggal() time.Time
}

type dataListAnggota struct {
	Anggota      []model.Anggota
	Cari         string
	FilterStatus string
	Limit        int

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

// GetList menampilkan seluruh anggota
func (h *Handler) GetList(w http.ResponseWriter, r *http.Request) {
	anggotaList, err := h.svc.Anggota.GetAll(r.Context())
	if err != nil {
		h.error(w, r, err)
		return
	}

	hal := h.halaman(r, "Anggota", "anggota")
	if r.URL.Query().Get("berhasil") == "1" {
		hal.Pesan = "Anggota berhasil didaftarkan"
	}
	h.render(w, http.StatusOK, "pages/anggota.html", dataListAnggota{
		Halaman: hal,
		Anggota: anggotaList,
	})
}

// GetListFiltered menampilkan seluruh anggota yang difilter
func (h *Handler) GetListFiltered(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cari := strings.TrimSpace(q.Get("q"))
	status := q.Get("status")

	// default value untuk pagination
	pageStr := q.Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	limitStr := q.Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 10
	}

	if limit > 100 {
		limit = 100
	}

	res, err := h.svc.Anggota.GetListFiltered(r.Context(), cari, status, page, limit)
	if err != nil {
		h.error(w, r, err)
		return
	}

	hal := h.halaman(r, "Anggota", "anggota")
	if q.Get("berhasil") == "1" {
		hal.Pesan = "Anggota Berhasil didaftarkan"
	}

	// menghitung total halaman page
	totalHalaman := (res.Total + limit - 1) / limit

	mulai, sampai := 0, 0
	if len(res.Data) > 0 {
		mulai = (page-1)*limit + 1
		sampai = mulai + len(res.Data) - 1
	}

	halamanPrev := page - 1
	if halamanPrev < 1 {
		halamanPrev = 1
	}

	halamanNext := page + 1
	if totalHalaman == 0 {
		halamanNext = 1
	} else if halamanNext > totalHalaman {
		halamanNext = totalHalaman
	}

	h.render(w, http.StatusOK, "pages/anggota.html", dataListAnggota{
		Anggota:      res.Data,
		Cari:         cari,
		FilterStatus: status,
		Limit:        limit,

		Total:  res.Total,
		Mulai:  mulai,
		Sampai: sampai,

		Halaman:        hal,
		CurrentHalaman: page,
		HalamanPrev:    halamanPrev,
		HalamanNext:    halamanNext,
		TotalHalaman:   totalHalaman,
		NomorHalaman:   buatNomorHalaman(page, totalHalaman),
	})
}

func buatNomorHalaman(current, total int) []int {
	if total == 0 {
		return nil
	}

	awal := current - 2
	akhir := current + 2

	if awal < 1 {
		awal = 1
	}

	if akhir > total {
		akhir = total
	}

	nomor := make([]int, 0, akhir-awal+1)

	for i := awal; i <= akhir; i++ {
		nomor = append(nomor, i)
	}

	return nomor
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
