package handler

import (
	"net/http"
)

// Beranda menampilkan halaman awal.
func (h *Handler) Beranda(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "pages/dashboard.html", h.halaman(r, "Dashboard", "dashboard"))
}
