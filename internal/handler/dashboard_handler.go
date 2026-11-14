package handler

import (
	"net/http"
)

// Beranda menampilkan halaman awal.
func (h *Handler) Beranda(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "pages/dashboard.html", h.halaman(r, "Dashboard", "dashboard"))
}

// Healthz dipakai untuk pengecekan bahwa server hidup.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
