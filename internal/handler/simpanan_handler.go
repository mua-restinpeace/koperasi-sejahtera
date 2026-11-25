package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type SimpananService interface {
	Riwayat(ctx context.Context, idAnggota int64) (service.RiwayatSimpanan, error)
	SetorWajib(ctx context.Context, idAnggota int64, in service.SetorWajibInput) error
	SetorSukarela(ctx context.Context, idAnggota int64, in service.SukarelaInput) error
	TarikSukarela(ctx context.Context, idAnggota int64, in service.SukarelaInput) error
	Batalkan(ctx context.Context, idTransaksi int64, alasan string) (int64, error)
}

type dataSimpanan struct {
	Halaman
	service.RiwayatSimpanan
	Error map[string]string // pesan kesalahan form, ditampilkan di bagian atas
}

// GetSimpanan menampilkan saldo, form setor/tarik, dan riwayat satu anggota
func (h *Handler) GetSimpanan(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	h.halamanSimpanan(w, r, id, http.StatusOK, nil, successMessage(r.URL.Query().Get("berhasil")))
}

func (h *Handler) SetorWajib(w http.ResponseWriter, r *http.Request) {
	h.ubahSimpanan(
		w,
		r,
		"wajib",
		func(ctx context.Context, id int64) error {
			return h.svc.Simpanan.SetorWajib(ctx, id, service.SetorWajibInput{Periode: r.PostFormValue("periode")})
		},
	)
}

func (h *Handler) SetorSukarela(w http.ResponseWriter, r *http.Request) {
	h.ubahSimpanan(
		w,
		r,
		"setor",
		func(ctx context.Context, id int64) error {
			return h.svc.Simpanan.SetorSukarela(ctx, id, service.SukarelaInput{Jumlah: r.PostFormValue("jumlah"), Keterangan: r.PostFormValue("keterangan")})
		},
	)
}

func (h *Handler) TarikSukarela(w http.ResponseWriter, r *http.Request) {
	h.ubahSimpanan(
		w,
		r,
		"tarik",
		func(ctx context.Context, id int64) error {
			return h.svc.Simpanan.TarikSukarela(ctx, id, service.SukarelaInput{Jumlah: r.PostFormValue("jumlah"), Keterangan: r.PostFormValue("keterangan")})
		},
	)
}

// BatalSimpanan membatalkan satu transaksi; {id} pada URL adalah ID transaksi.
func (h *Handler) BatalSimpanan(w http.ResponseWriter, r *http.Request) {
	idTrx, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	idAnggota, err := h.svc.Simpanan.Batalkan(r.Context(), idTrx, r.PostFormValue("alasan"))
	h.hasilSimpanan(w, r, idAnggota, "batal", err)
}

func (h *Handler) halamanSimpanan(w http.ResponseWriter, r *http.Request, id int64, status int, galat map[string]string, pesan string) {
	d, err := h.svc.Simpanan.Riwayat(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrAnggotaTidakDitemukan):
		h.notFound(w)
	case err != nil:
		h.error(w, r, err)
	default:
		hal := h.halaman(r, "Simpanan Anggota", "simpanan")
		hal.Pesan = pesan
		h.render(w, status, "pages/simpanan_detail.html", dataSimpanan{
			Halaman:         hal,
			RiwayatSimpanan: d,
			Error:           galat,
		})
	}
}

// ubahSimpanan menjalankan aksi setor/tarik untuk anggota {id} lalu menanggapi hasilnya.
func (h *Handler) ubahSimpanan(w http.ResponseWriter, r *http.Request, kode string, aksi func(ctx context.Context, id int64) error) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	h.hasilSimpanan(w, r, id, kode, aksi(r.Context(), id))
}

func (h *Handler) hasilSimpanan(w http.ResponseWriter, r *http.Request, idAnggota int64, kode string, err error) {
	var ve *service.ValidasiError
	switch {
	case err == nil:
		http.Redirect(w, r, fmt.Sprintf("/simpanan/%d?berhasil=%s", idAnggota, kode), http.StatusSeeOther)
	case errors.Is(err, service.ErrAnggotaTidakDitemukan), errors.Is(err, service.ErrTransaksiTidakDitemukan):
		h.notFound(w)
	case errors.As(err, &ve):
		h.halamanSimpanan(w, r, idAnggota, http.StatusUnprocessableEntity, ve.Pesan, "")
	case errors.Is(err, service.ErrAnggotaSudahKeluar), errors.Is(err, service.ErrSudahDibatalkan), errors.Is(err, service.ErrPokokTidakBisaDibatalkan):
		h.halamanSimpanan(w, r, idAnggota, http.StatusConflict, map[string]string{"aturan": err.Error() + "."}, "")
	default:
		h.error(w, r, err)
	}
}
