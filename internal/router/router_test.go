package router_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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

type fakeAnggota struct {
	diterima []service.DaftarAnggotaInput
	galat    error

	hasilFilter service.PaginatedAnggotaResponse
	errFilter   error

	cariFilter   string
	statusFilter string
	pageFilter   int
	limitFilter  int
}

func (f *fakeAnggota) Register(_ context.Context, in service.DaftarAnggotaInput) (int64, error) {
	if f.galat != nil {
		return 0, f.galat
	}
	f.diterima = append(f.diterima, in)
	return 1, nil
}
func (*fakeAnggota) GetAll(context.Context) ([]model.Anggota, error) {
	return []model.Anggota{{NomorAnggota: "A-001", NamaLengkap: "Siti <b>Aminah</b>", TanggalBergabung: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Status: "aktif"}}, nil
}

func (f *fakeAnggota) GetListFiltered(
	_ context.Context,
	cari, status string,
	page, limit int,
) (service.PaginatedAnggotaResponse, error) {

	f.cariFilter = cari
	f.statusFilter = status
	f.pageFilter = page
	f.limitFilter = limit

	return f.hasilFilter, f.errFilter
}

func (*fakeAnggota) FormatTanggal() time.Time { return time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) }

func siap(t *testing.T) (http.Handler, *fakeAuth, *fakeAnggota) {
	t.Helper()
	cfg := config.Default()
	cfg.CookieSecure = true
	auth, ang := &fakeAuth{}, &fakeAnggota{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := handler.NewHandler(cfg, log, templates.FS, handler.Service{Auth: auth, Anggota: ang})
	if err != nil {
		t.Fatalf("handler.New: %v", err)
	}
	return router.NewRouter(h, auth, log), auth, ang
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
	h, _, _ := siap(t)
	for _, p := range []string{"/", "/anggota", "/anggota/baru"} {
		w := kirim(h, http.MethodGet, p, nil, "")
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
			t.Errorf("GET %s: status=%d location=%q", p, w.Code, w.Header().Get("Location"))
		}
	}
	if w := kirim(h, http.MethodPost, "/anggota", url.Values{"nomor_anggota": {"1"}}, ""); w.Code != http.StatusSeeOther {
		t.Errorf("POST /anggota tanpa login: status=%d", w.Code)
	}
	if w := kirim(h, http.MethodGet, "/healthz", nil, ""); w.Code != http.StatusOK {
		t.Errorf("healthz: status=%d", w.Code)
	}
}

func TestLogin(t *testing.T) {
	h, _, _ := siap(t)

	w := kirim(h, http.MethodGet, "/login", nil, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `name="password"`) {
		t.Errorf("form login: status=%d", w.Code)
	}

	w = kirim(h, http.MethodPost, "/login", url.Values{"username": {"ani"}, "password": {"salah"}}, "")
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Username atau kata sandi salah") {
		t.Errorf("login salah: status=%d", w.Code)
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

func TestSimpanAnggota(t *testing.T) {
	h, _, ang := siap(t)
	isian := url.Values{"nomor_anggota": {"A-002"}, "nama_lengkap": {"Budi"}, "no_telepon": {""}, "tanggal_bergabung": {"2026-11-01"}}

	// Tanpa token CSRF.
	if w := kirim(h, http.MethodPost, "/anggota", isian, "token-ani"); w.Code != http.StatusForbidden || len(ang.diterima) != 0 {
		t.Errorf("tanpa csrf: status=%d diterima=%d", w.Code, len(ang.diterima))
	}

	// Dengan token benar.
	isian.Set("csrf_token", "csrf-ani")
	w := kirim(h, http.MethodPost, "/anggota", isian, "token-ani")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/anggota?berhasil=1" {
		t.Fatalf("simpan: status=%d location=%q", w.Code, w.Header().Get("Location"))
	}
	if len(ang.diterima) != 1 || ang.diterima[0].NomorAnggota != "A-002" || ang.diterima[0].TanggalBergabung != "2026-11-01" {
		t.Errorf("diterima = %+v", ang.diterima)
	}

	// Validasi gagal: form tampil ulang dengan isian dan pesan.
	ang.galat = &service.ValidasiError{Pesan: map[string]string{"nomor_anggota": "sudah dipakai anggota lain"}}
	w = kirim(h, http.MethodPost, "/anggota", isian, "token-ani")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("validasi: status=%d", w.Code)
	}
	if b := w.Body.String(); !strings.Contains(b, "sudah dipakai anggota lain") || !strings.Contains(b, `value="A-002"`) {
		t.Error("form harus menampilkan pesan dan isian sebelumnya")
	}

	// Kesalahan tak terduga: pesan umum, tanpa membocorkan detail.
	ang.galat = io.ErrUnexpectedEOF
	w = kirim(h, http.MethodPost, "/anggota", isian, "token-ani")
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "EOF") {
		t.Errorf("galat server: status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestLogout(t *testing.T) {
	h, auth, _ := siap(t)

	if w := kirim(h, http.MethodPost, "/logout", url.Values{}, "token-ani"); w.Code != http.StatusForbidden {
		t.Errorf("logout tanpa csrf: status=%d", w.Code)
	}

	w := kirim(h, http.MethodPost, "/logout", url.Values{"csrf_token": {"csrf-ani"}}, "token-ani")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
		t.Fatalf("logout: status=%d location=%q", w.Code, w.Header().Get("Location"))
	}
	if len(auth.keluar) != 1 || auth.keluar[0] != "token-ani" {
		t.Errorf("Logout service = %v", auth.keluar)
	}
	cs := w.Result().Cookies()
	if len(cs) != 1 || cs[0].MaxAge >= 0 {
		t.Errorf("cookie harus dihapus: %+v", cs)
	}
}

func TestListFilteredAnggota(t *testing.T) {
	h, _, ang := siap(t)

	ang.hasilFilter = service.PaginatedAnggotaResponse{
		Data: []model.Anggota{
			{
				NomorAnggota: "A-001",
				NamaLengkap:  "Siti Aminah",
				Status:       "aktif",
			},
		},
		Total:    1,
		Page:     1,
		PageSize: 10,
	}

	w := kirim(
		h,
		http.MethodGet,
		"/anggota?q=A-001&status=aktif&page=1&limit=10",
		nil,
		"token-ani",
	)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "Siti Aminah") {
		t.Error("halaman harus menampilkan Siti Aminah")
	}

	if ang.cariFilter != "A-001" ||
		ang.statusFilter != "aktif" ||
		ang.pageFilter != 1 ||
		ang.limitFilter != 10 {
		t.Errorf("parameter filter tidak diteruskan dengan benar")
	}
}
