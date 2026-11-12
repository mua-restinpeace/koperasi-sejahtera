package router_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/config"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/handler"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/router"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
	"github.com/mua-restinpeace/koperasi-sejahtera/templates"
)

// fakeAuth memenuhi handler.AuthLayanan dan middleware.Autentikator.
type fakeAuth struct{ keluar []string }

func (*fakeAuth) Login(_ context.Context, u, p string) (string, model.Sesi, error) {
	if u == "ani" && p == "rahasia-panjang" {
		return "token-ani", model.Sesi{KedaluwarsaPada: time.Now().Add(time.Hour)}, nil
	}
	return "", model.Sesi{}, service.ErrKredensialSalah
}
func (f *fakeAuth) Logout(_ context.Context, t string) error {
	f.keluar = append(f.keluar, t)
	return nil
}
func (*fakeAuth) Authenticate(_ context.Context, t string) (service.Identitas, error) {
	if t == "token-ani" {
		return service.Identitas{IDPengguna: 1, Username: "ani", NamaLengkap: "Ani Lestari", CSRFToken: "csrf-ani"}, nil
	}
	return service.Identitas{}, service.ErrSesiTidakValid
}

func siap(t *testing.T) (http.Handler, *fakeAuth) {
	t.Helper()
	cfg := config.Default()
	cfg.CookieSecure = true
	auth := &fakeAuth{}
	// log := slog.New(slog.NewTextHandler(io.Discard, nil))
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	h, err := handler.NewHandler(cfg, log, templates.FS, handler.Service{Auth: auth})
	if err != nil {
		t.Fatalf("handler.New: %v", err)
	}
	return router.NewRouter(h, auth, log), auth
}

func kirim(h http.Handler, method, path string, form url.Values, cookie string) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, path, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: "session_token", Value: cookie})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRute_TanpaLoginDialihkan(t *testing.T) {
	h, _ := siap(t)
	for _, p := range []string{"/"} {
		w := kirim(h, http.MethodGet, p, nil, "")
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
			t.Errorf("GET %s: status=%d location=%q", p, w.Code, w.Header().Get("Location"))
		}
	}
}

func TestLogin(t *testing.T) {
	h, _ := siap(t)

	w := kirim(h, http.MethodGet, "/login", nil, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `name="password"`) {
		t.Errorf("form login: status=%d, body=%s", w.Code, w.Body)
	}

	w = kirim(h, http.MethodPost, "/login", url.Values{"username": {"ani"}, "password": {"salah"}}, "")
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Username atau kata sandi salah") {
		t.Errorf("login salah: status=%d, body=%s", w.Code, w.Body)
	}
	if len(w.Result().Cookies()) != 0 {
		t.Error("login gagal tidak boleh memasang cookie")
	}

	w = kirim(h, http.MethodPost, "/login", url.Values{"username": {"ani"}, "password": {"rahasia-panjang"}}, "")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("login benar: status=%d location=%q", w.Code, w.Header().Get("Location"))
	}
	cs := w.Result().Cookies()
	if len(cs) != 1 {
		t.Fatalf("cookie = %v", cs)
	}
	c := cs[0]
	if c.Name != "session_token" || c.Value != "token-ani" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Errorf("cookie = %+v", c)
	}

	// Sudah login: halaman login dialihkan ke beranda.
	if w := kirim(h, http.MethodGet, "/login", nil, "token-ani"); w.Code != http.StatusSeeOther {
		t.Errorf("login saat sudah login: status=%d", w.Code)
	}
}
