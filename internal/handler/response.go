package handler

import "net/http"

// internalError mencatat kesalahan tak terduga dan menampilkan pesan umum ke pengguna
func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("handler error", "method", r.Method, "path", r.URL.Path, "error", err)
	http.Error(w, "Terjadi kesalahan pada server", http.StatusInternalServerError)
}
