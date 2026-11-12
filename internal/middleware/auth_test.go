package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type fakeAuth struct{}

func (fakeAuth) Authenticate(_ context.Context, token string) (service.Identitas, error) {
	if token == "valid" {
		return service.Identitas{IDPengguna: 1, Username: "ani", CSRFToken: "csrf-abc"}, nil
	}
	return service.Identitas{}, service.ErrSesiTidakValid
}

func ok(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

func lindungi() http.Handler {
	return Sesi(fakeAuth{})(RequireLogin(CSRF(http.HandlerFunc(ok))))
}

func kirim(h http.Handler, method, cookie, csrf string) *httptest.ResponseRecorder {
	var body *strings.Reader
	if csrf != "" {
		body = strings.NewReader(url.Values{NamaFieldCSRF: {csrf}}.Encode())
	} else {
		body = strings.NewReader("")
	}
	r := httptest.NewRequest(method, "/x", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: NamaCookieSesi, Value: cookie})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestWajibLogin(t *testing.T) {
	tests := []struct {
		nama, cookie string
		status       int
	}{
		{"tanpa cookie", "", http.StatusSeeOther},
		{"cookie tidak valid", "palsu", http.StatusSeeOther},
		{"cookie valid", "valid", http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			w := kirim(lindungi(), http.MethodGet, tt.cookie, "")
			if w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}
			if tt.status == http.StatusSeeOther && w.Header().Get("Location") != "/login" {
				t.Errorf("Location = %q", w.Header().Get("Location"))
			}
		})
	}
}

func TestCSRF(t *testing.T) {
	tests := []struct {
		nama, method, csrf string
		status             int
	}{
		{"GET tidak perlu token", http.MethodGet, "", http.StatusNoContent},
		{"POST tanpa token", http.MethodPost, "", http.StatusForbidden},
		{"POST token salah", http.MethodPost, "salah", http.StatusForbidden},
		{"POST token benar", http.MethodPost, "csrf-abc", http.StatusNoContent},
		{"DELETE tanpa token", http.MethodDelete, "", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			if w := kirim(lindungi(), tt.method, "valid", tt.csrf); w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestSecureHeadersDanBatasBody(t *testing.T) {
	h := SecureHeaders(BatasBody(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "terlalu besar", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("a="+strings.Repeat("x", 100)))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
	for _, k := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Referrer-Policy"} {
		if w.Header().Get(k) == "" {
			t.Errorf("header %s tidak terpasang", k)
		}
	}
}
