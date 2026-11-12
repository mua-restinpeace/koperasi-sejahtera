package middleware

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

// NamaCookieSesi adalah nama cookie yang membawa token sesi
const NamaCookieSesi = "session_token"

// NamaFieldCSRF adlah nama field form tersembunyi yang membawa token CSRF
const NamaFieldCSRF = "csrf_token"

// Authenticator mengubah token cookie menjadi identitas anggota
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (service.Identitas, error)
}

// Sesi membaca cookie sesi dan bila valid menaruh Identitas ke context
func Sesi(a Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// if c, err := r.Cookie(NamaCookieSesi); err == nil {
			// 	if id, err := a.Authenticate(r.Context(), c.Value); err == nil {
			// 		r = r.WithContext(service.DenganIdentitas(r.Context(), id))
			// 	}
			// }
			c, err := r.Cookie(NamaCookieSesi)

			if err != nil {
				fmt.Println("SESSION COOKIE NOT FOUND:", err)
			} else {
				id, err := a.Authenticate(r.Context(), c.Value)

				if err != nil {
					fmt.Println("SESSION AUTHENTICATION FAILED:", err)
				} else {
					fmt.Println("SESSION AUTHENTICATED:", id.Username)
					r = r.WithContext(
						service.DenganIdentitas(r.Context(), id),
					)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireLogin mengalihkan ke /login bila belum ada identitas pada context
func RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := service.IdentitasDari(r.Context()); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			fmt.Println("redirecting to /login")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSRF menolat permintaan yang mengubah data
// (selain GET/HEAD/OPTIONS) bila token pada form tidak sama dengan token sesi
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		id, ok := service.IdentitasDari(r.Context())
		if !ok || id.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(r.PostFormValue(NamaFieldCSRF)), []byte(id.CSRFToken)) != 1 {
			http.Error(w, "Permintaan ditolak (token keamanan tidak valid). Muat ulang halaman ini lalu coba lagi", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecureHeaders memasang header keamanan dasar pada setiap respons.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// BatasBody membatasi ukuran body permintaan agar tidak menghabiskan memori.
func BatasBody(maks int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maks)
			next.ServeHTTP(w, r)
		})
	}
}
