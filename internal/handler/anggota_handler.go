package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type AnggotaService interface {
	Register(ctx context.Context, in service.DaftarAnggotaInput) (int64, error)
	GetAnggota(ctx context.Context, id int64) (service.GetAnggota, error)
	GetListFiltered(ctx context.Context, cari, status string, page, limit int) (service.PaginatedAnggotaResponse, error)
	GetByID(ctx context.Context, id int64) (model.Anggota, error)
	Update(ctx context.Context, id int64, in service.UpdateAnggotaInput) error
	FormatTanggal() time.Time
	PertinjauResign(ctx context.Context, id int64) (service.RincianResign, error)
	Resign(ctx context.Context, id int64) (model.SaldoSimpanan, error)
}

type dataListAnggota struct {
	Anggota      []model.Anggota
	Cari         string
	FilterStatus string
	Limit        int
	Path         string

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

type dataDetailAnggota struct {
	Halaman
	service.GetAnggota
}

type dataFormAnggota struct {
	Halaman
	Isian service.DaftarAnggotaInput
	Error map[string]string
}

type dataFormUpdateAnggota struct {
	Halaman
	Anggota model.Anggota // nomor anggota ditampilkan tetapi tidak dapat diubah
	Isian   service.UpdateAnggotaInput
	Error   map[string]string
}

type dataAnggotaResign struct {
	Halaman
	service.RincianResign
	Alasan string
}

// successMessage menerjemahkan query ?berhasil=.. pada URL menjadi pesan
func successMessage(kode string) string {
	switch kode {
	case "1":
		return "Anggota berhasil didaftarkan"
	case "ubah":
		return "Data anggota berhasil diubah"
	case "keluar":
		return "Anggota berhasil dikeluarkan dan simpanannya dicatat sebagai dikembalikan"
	case "wajib":
		return "Setoran simpanan wajib berhasil dicatat"
	case "setor":
		return "Setoran simpanan sukarela berhasil dicatat"
	case "tarik":
		return "Penarikan simpanan sukarela berhasil dicatat"
	case "batal":
		return "Transaksi simpanan berhasil dibatalkan"
	default:
		return ""
	}
}

// idDariPath membaca {id} dari URL; ok=false bila bukan bilangan bulat positif.
func idDariPath(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func urlDetailAnggota(id int64, berhasil string) string {
	u := fmt.Sprintf("/anggota/%d", id)
	if berhasil != "" {
		u += "?berhasil=" + berhasil
	}

	return u
}

// DetailAnggota menampilkan profil, saldo simpanan, dan tombol aksi satu anggota
func (h *Handler) DetailAnggota(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	d, err := h.svc.Anggota.GetAnggota(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrAnggotaTidakDitemukan):
		h.notFound(w)
	case err != nil:
		h.error(w, r, err)
	default:
		hal := h.halaman(r, "Detail Anggota", "anggota")
		hal.Pesan = successMessage(r.URL.Query().Get("berhasil"))
		h.render(w, http.StatusOK, "pages/anggota_detail.html", dataDetailAnggota{Halaman: hal, GetAnggota: d})
	}
}

// GetListFiltered menampilkan seluruh anggota yang difilter
func (h *Handler) GetListFiltered(w http.ResponseWriter, r *http.Request) {
	path, judul, menu := "/anggota", "Anggoat", "anggota"
	if r.URL.Path == "/simpanan" {
		path, judul, menu = "/simpanan", "Simpanan", "simpanan"
	}

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

	hal := h.halaman(r, judul, menu)
	if q.Get("berhasil") == "1" {
		hal.Pesan = successMessage(q.Get("berhasil"))
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
		Path:         path,

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

// FormUpdate menampilkan form ubah data anggota
func (h *Handler) FormUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	a, err := h.svc.Anggota.GetByID(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrAnggotaTidakDitemukan):
		h.notFound(w)
	case err != nil:
		h.error(w, r, err)
	default:
		h.render(w, http.StatusOK, "pages/anggota_update.html",
			dataFormUpdateAnggota{
				Halaman: h.halaman(r, "Update Anggota", "anggota"),
				Anggota: a,
				Isian:   service.UpdateAnggotaInput{NamaLengkap: a.NamaLengkap, NoTelepon: a.NoTelepon},
			})
	}
}

// SaveUpdateAnggota memproses form update data
func (h *Handler) SaveUpdateAnggota(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	in := service.UpdateAnggotaInput{
		NamaLengkap: r.PostFormValue("nama_lengkap"),
		NoTelepon:   r.PostFormValue("no_telepon"),
	}

	err := h.svc.Anggota.Update(r.Context(), id, in)
	var ve *service.ValidasiError
	switch {
	case errors.Is(err, service.ErrAnggotaTidakDitemukan):
		h.notFound(w)
	case errors.As(err, &ve):
		a, errCari := h.svc.Anggota.GetByID(r.Context(), id) // untuk menampilkan nomor anggota
		if errCari != nil {
			h.error(w, r, errCari)
			return
		}

		h.render(w, http.StatusUnprocessableEntity, "pages/anggota_update.html",
			dataFormUpdateAnggota{
				Halaman: h.halaman(r, "Update Anggota", "anggota"),
				Anggota: a,
				Isian:   in,
				Error:   ve.Pesan,
			})
	case err != nil:
		h.error(w, r, err)
	default:
		http.Redirect(w, r, urlDetailAnggota(id, "ubah"), http.StatusSeeOther)
	}
}

// FormAnggotaResign menampilkan halaman konfiramsi keluar
// simpanan yang akan dikembalikan, atau alasan anggota belum boleh keluar
func (h *Handler) FormAnggoaResign(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	h.resignConfirmation(w, r, id, http.StatusOK)
}

// resignConfirmation merender halaman konfirmasi keluar dengan status HTTP yang diminta
func (h *Handler) resignConfirmation(w http.ResponseWriter, r *http.Request, id int64, status int) {
	rincian, err := h.svc.Anggota.PertinjauResign(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrAnggotaTidakDitemukan):
		h.notFound(w)
		return
	case err != nil:
		h.error(w, r, err)
		return
	}

	data := dataAnggotaResign{
		Halaman:       h.halaman(r, "Anggota Resign", "anggota"),
		RincianResign: rincian,
	}

	switch {
	case errors.Is(rincian.Penghalang, service.ErrMasihAdaPinjamanAktif):
		data.Alasan = "Anggota masih memiliki pinjaman aktif. Lunasi peminjamannya terlebh dahulu sebelum anggoa dikeluarkan."
	case errors.Is(rincian.Penghalang, service.ErrAnggotaSudahKeluar):
		data.Alasan = "Anggota sudah sudah berstatus keluar."
	}
	h.render(w, status, "pages/anggota_keluar.html", data)
}

// AnggotaResign mengeluarkan anggota
// bila ditolak, halaman konfirmasi ditampilkan lagi dengan alasannya
func (h *Handler) AnggotaResign(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	_, err := h.svc.Anggota.Resign(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrAnggotaTidakDitemukan):
		h.notFound(w)
	case errors.Is(err, service.ErrMasihAdaPinjamanAktif), errors.Is(err, service.ErrAnggotaSudahKeluar):
		h.resignConfirmation(w, r, id, http.StatusConflict)
	case err != nil:
		h.error(w, r, err)
	default:
		http.Redirect(w, r, urlDetailAnggota(id, "keluar"), http.StatusSeeOther)
	}
}
