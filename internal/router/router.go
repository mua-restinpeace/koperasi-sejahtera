package router

import (
	"log/slog"
	"net/http"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/handler"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/middleware"
	"github.com/mua-restinpeace/koperasi-sejahtera/static"
)

// maksBody adalah batas ukuran body permintaan (1 MiB)
const maksBody = 1 << 20

// router dipakai setiap fitur untuk mendaftarkan URLnya
type router struct {
	mux *http.ServeMux
	h   *handler.Handler
}

// Public mendaftarkan rute yangbisa diakses tanpa login
func (r *router) Public(pola string, fn http.HandlerFunc) {
	r.mux.Handle(pola, fn)
}

// Protected mendaftarkan rute yang wajib login
// untuk method selain GET, token SCSRF pada form diperiksa
// SEMUA rute fitur memakai ini
func (r *router) Protected(pola string, fn http.HandlerFunc) {
	r.mux.Handle(pola, middleware.RequireLogin(middleware.CSRF(fn)))
}

// NewRouter mengembalikan seluruh rute aplikas
func NewRouter(h *handler.Handler, auth middleware.Authenticator, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Static files: tidak memerlukan login atau CSRF.
	mux.Handle(
		"GET /static/",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.FS(static.FS)),
		),
	)

	r := &router{mux: mux, h: h}

	r.Public("GET /login", h.FormLogin)
	r.Public("POST /login", h.Login)
	r.Protected("GET /{$}", h.Beranda)
	r.Protected("POST /logout", h.Logout)

	// urutan dari luar ke dalam: header, log, batas body, sesi, lalu mux.
	var out http.Handler = mux
	out = middleware.Sesi(auth)(out)
	out = middleware.BatasBody(maksBody)(out)
	out = middleware.Logging(log)(out)
	out = middleware.SecureHeaders(out)

	return out
}
