package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/config"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
	"github.com/mua-restinpeace/koperasi-sejahtera/templates"
)

// Memastikan seluruh template ter-parse dan dashboard ter-render dengan layout.
func TestBeranda_Render(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHandler(config.Config{}, log, templates.FS, Service{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(service.DenganIdentitas(req.Context(), service.Identitas{
		IDPengguna: 1, Username: "admin", NamaLengkap: "Budi Pratama", CSRFToken: "tok",
	}))
	rec := httptest.NewRecorder()
	h.Beranda(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Dashboard", "Angsuran terlambat", "Rp482.000.000", ">BP<", "/laporan", "komponen.css"} {
		if !strings.Contains(body, want) {
			t.Errorf("body tidak memuat %q", want)
		}
	}
	if out := os.Getenv("KOPERASI_PREVIEW_OUT"); out != "" {
		_ = os.WriteFile(out, rec.Body.Bytes(), 0o644)
	}
}
