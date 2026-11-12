package handler

import (
	"bytes"
	"net/http"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

// Halaman adalah daftar dasar yang dibutuhkan layout.html. Struct data setiap
// halaman menyematkan Halaman, lalu dibuat lewat h.halaman(r, "Judul")
type Halaman struct {
	Judul     string
	MenuAktif string
	Pengguna  *service.Identitas
	CSRF      string
	Pesan     string
}

// halaman membuat halaman dari permintaan (identitas dan token CSRF diambil dari context)
func (h *Handler) halaman(r *http.Request, judul, menuAktif string) Halaman {
	hal := Halaman{
		Judul:     judul,
		MenuAktif: menuAktif,
	}
	if id, ok := service.IdentitasDari(r.Context()); ok {
		hal.Pengguna = &id
		hal.CSRF = id.CSRFToken
	}

	return hal
}

// render merender satu halaman ke buffer lebih dulu supaya kesalahan
// template tidak menghasilkan response setengah jadi
func (h *Handler) render(w http.ResponseWriter, status int, nama string, data any) {
	t, ok := h.tmpl[nama]
	if !ok {
		h.log.Error("template tidak ditemukan", "halaman", nama)
		http.Error(w, "Terjadi kesalahan pada server", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		h.log.Error("render tamplate", "halaman", nama, "error", err)
		http.Error(w, "Terjadi kesalahan pada server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}
