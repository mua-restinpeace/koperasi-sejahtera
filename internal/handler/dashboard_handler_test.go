package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

func dashboardUji() service.Dashboard {
	bulan := func(y int, m time.Month, jasa, denda int64) service.PendapatanBulan {
		return service.PendapatanBulan{Bulan: time.Date(y, m, 1, 0, 0, 0, 0, time.UTC), Jasa: jasa, Denda: denda}
	}
	return service.Dashboard{
		Tanggal: tanggalUji("2027-02-11"),
		Bulan:   tanggalUji("2027-02-01"),
		Ringkasan: service.RingkasanDashboard{
			AnggotaAktif: 12, AnggotaBaru: 2, PinjamanAktif: 3,
			SisaPinjaman: 4_702_000, TotalSimpanan: 9_300_000, PendapatanJasa: 120_000, PendapatanDenda: 56_000,
		},
		Terlambat: []service.BarisTunggakan{
			{IDAnggota: 2, NomorAnggota: "KKS-0002", NamaAnggota: "Rudi Hartono", IDPinjaman: 7, Urutan: 2, JatuhTempo: tanggalUji("2027-01-10"), HariTerlambat: 32, SisaPokok: 702_000, DendaBerjalan: 54_000},
		},
		TotalTelat:    1,
		WajibBelum:    []model.AnggotaWajibBelum{{ID: 5, NomorAnggota: "KKS-0005", NamaAnggota: "Maya Putri"}},
		TotalWajib:    4,
		WajibPerBulan: 100_000,
		Grafik:        []service.PendapatanBulan{bulan(2027, 1, 120_000, 82_000), bulan(2027, 2, 120_000, 56_000)},
	}
}

func TestBeranda_DataAsli(t *testing.T) {
	f := &fakeLaporan{dashboard: dashboardUji()}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).Beranda(rec, reqLogin("/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	wajibMemuat(t, body,
		"Dashboard", "Februari 2027", ">BP<", "komponen.css", "/laporan",
		"Rp4.702.000", "Rp9.300.000", "Rp120.000", "Rp56.000", ">12<", "+2 bulan ini", "3 pinjaman aktif",
		"Rudi Hartono", "10 Jan 2027", "32 hari", "Rp756.000", "/laporan/pinjaman/7",
		"Maya Putri", "KKS-0005", "Rp100.000", "/simpanan/5", "4 anggota",
		"Jan", "Feb",
	)
	for _, tidak := range []string{"Data contoh", "Andi Pratama", "Rp482.000.000"} {
		if strings.Contains(body, tidak) {
			t.Errorf("dashboard masih memuat data contoh %q", tidak)
		}
	}
	simpanPratinjau(t, "KOPERASI_PREVIEW_OUT", body)
}

func TestBeranda_Kosong(t *testing.T) {
	f := &fakeLaporan{dashboard: service.Dashboard{Bulan: tanggalUji("2027-02-01")}}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).Beranda(rec, reqLogin("/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	wajibMemuat(t, rec.Body.String(), "Tidak ada angsuran terlambat", "Semua simpanan wajib sudah dibayar", "Rp0")
}

func TestBeranda_GalatTidakBocor(t *testing.T) {
	f := &fakeLaporan{errDashboard: errors.New("rahasia: password db salah")}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).Beranda(rec, reqLogin("/"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "rahasia") {
		t.Error("detail galat bocor ke pengguna")
	}
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{}).Healthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("healthz: %d %q", rec.Code, rec.Body.String())
	}
}
