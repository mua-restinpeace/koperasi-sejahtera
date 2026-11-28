package router_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/config"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/handler"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/router"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
	"github.com/mua-restinpeace/koperasi-sejahtera/templates"
)

type fakeLaporanKosong struct{}

func (fakeLaporanKosong) Pendapatan(context.Context, service.PendapatanInput) (service.LaporanPendapatan, error) {
	return service.LaporanPendapatan{}, nil
}
func (fakeLaporanKosong) Tunggakan(context.Context, service.TunggakanInput) (service.LaporanTunggakan, error) {
	return service.LaporanTunggakan{}, nil
}

func (fakeLaporanKosong) DaftarPinjaman(context.Context) ([]model.PinjamanKartu, error) {
	return nil, nil
}
func (fakeLaporanKosong) KartuPinjaman(context.Context, service.KartuInput) (service.KartuPinjaman, error) {
	return service.KartuPinjaman{}, nil
}

func (fakeLaporanKosong) Dashboard(context.Context) (service.Dashboard, error) {
	return service.Dashboard{Bulan: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}, nil
}

func TestLaporan_Rute(t *testing.T) {
	cfg := config.Default()
	auth := &fakeAuth{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := handler.NewHandler(cfg, log, templates.FS, handler.Service{Auth: auth, Laporan: fakeLaporanKosong{}})
	if err != nil {
		t.Fatalf("handler.New: %v", err)
	}
	r := router.NewRouter(h, auth, log)

	for _, path := range []string{"/laporan", "/laporan/pinjaman", "/laporan/pinjaman/1", "/laporan/pinjaman/1/csv", "/laporan/pendapatan", "/laporan/pendapatan/csv", "/laporan/tunggakan", "/laporan/tunggakan/csv"} {
		t.Run("tanpa login "+path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
				t.Errorf("status = %d, want redirect ke login", rec.Code)
			}
		})

		t.Run("login "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.AddCookie(&http.Cookie{Name: "session_token", Value: "token-ani"})
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rec.Code)
			}
		})
	}
}
