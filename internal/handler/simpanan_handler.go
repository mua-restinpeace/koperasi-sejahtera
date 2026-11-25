package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type SimpananService interface {
	Riwayet(ctx context.Context, idAnggota int64) (service.RiwayatSimpanan, error)
}

type dataSimpanan struct {
	Halaman
	service.RiwayatSimpanan
	Error map[string]string // pesan kesalahan form, ditampilkan di bagian atas
}

// GetSimpanan menampilkan saldo, form setor/tarik, dan riwayat satu anggota
func (h *Handler) GetSimpanan(w http.ResponseWriter, r *http.Request){
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	h.halamanSimpanan(w, r, id, http.StatusOK, nil, successMessage(r.URL.Query().Get("berhasil")))
}


func (h *Handler) halamanSimpanan(w http.ResponseWriter, r *http.Request, id int64, status int, galat map[string]string, pesan string){
	d, err := h.svc.Simpanan.Riwayet(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrAnggotaTidakDitemukan):
		h.notFound(w)
	case err != nil:
		h.error(w, r, err)
	default:
		hal := h.halaman(r, "Simpanan Anggota", "simpanan")
		hal.Pesan = pesan
		h.render(w, status, "pages/simpanan_detail.html", dataSimpanan{
			Halaman: hal,
			RiwayatSimpanan: d,
			Error: galat,
		})
	}
}