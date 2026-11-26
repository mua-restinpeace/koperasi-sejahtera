package handler

import (
	"net/http"
)

// Beranda menampilkan dashboard. Sementara memakai data contoh sampai service
// laporan tersedia (lihat contohDashboard di dashboard_view.go).
func (h *Handler) Beranda(w http.ResponseWriter, r *http.Request) {
	d := contohDashboard()
	d.Halaman = h.halaman(r, "Dashboard", "dashboard")
	h.render(w, http.StatusOK, "pages/dashboard.html", d)
}

// Healthz dipakai untuk pengecekan bahwa server hidup.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
