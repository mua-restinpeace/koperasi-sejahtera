package handler

import (
	"net/http"
)

// Beranda menampilkan dashboard dengan angka asli dari database
// (lihat LaporanService.Dashboard).
func (h *Handler) Beranda(w http.ResponseWriter, r *http.Request) {
	hasil, err := h.svc.Laporan.Dashboard(r.Context())
	if err != nil {
		h.error(w, r, err)
		return
	}

	d := tampilanDashboard(hasil)
	d.Halaman = h.halaman(r, "Dashboard", "dashboard")
	h.render(w, http.StatusOK, "pages/dashboard.html", d)
}

// Healthz dipakai untuk pengecekan bahwa server hidup.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
