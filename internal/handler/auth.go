package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/middleware"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type AuthService interface {
	Login(ctx context.Context, username, password string) (string, model.Sesi, error)
	Logout(ctx context.Context, token string) error
}

type Service struct {
	Auth AuthService
}

type dataLogin struct {
	Halaman
	Username string
	Error    string
}

// FormLogin menampilkan halaman login; bila sudah login, langsung ke beranda
func (h *Handler) FormLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := service.IdentitasDari(r.Context()); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	h.render(w, http.StatusOK, "pages/login.html", dataLogin{Halaman: h.halaman(r, "Masuk", "login")})
}

// Login memeriksa kredensial, memasang cookie sesi, lalu mengalihkan ke beranda.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	token, sesi, err := h.svc.Auth.Login(r.Context(), username, r.PostFormValue("password"))
	if errors.Is(err, service.ErrKredensialSalah) {
		h.render(w, http.StatusUnauthorized, "pages/login.html", dataLogin{
			Halaman:  h.halaman(r, "Masuk", "login"),
			Username: username,
			Error:    "Username atau kata sandi salah.",
		})
		return
	}
	if err != nil {
		h.internalError(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     middleware.NamaCookieSesi,
		Value:    token,
		Path:     "/",
		Expires:  sesi.KedaluwarsaPada,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout menghapus sesi di server dan cookie di peramban. Rute ini POST dan
// berada di balik CSRF.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(middleware.NamaCookieSesi); err == nil {
		if err := h.svc.Auth.Logout(r.Context(), c.Value); err != nil {
			h.log.Error("logout", "error", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.NamaCookieSesi,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
