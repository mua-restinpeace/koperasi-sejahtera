package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

func (f *fakeLaporanRepo) WajibBelumPeriode(_ context.Context, _ database.DBTX, periode time.Time) ([]model.AnggotaWajibBelum, error) {
	f.periodeMinta = periode
	return f.wajibPeriode, f.galat
}

func belumUji() []model.AnggotaWajibBelum {
	return []model.AnggotaWajibBelum{
		{ID: 2, NomorAnggota: "KKS-0002", NamaAnggota: "Dewi Anggraini"},
		{ID: 5, NomorAnggota: "KKS-0005", NamaAnggota: "=Maya"},
	}
}

func TestHitungLaporanWajib(t *testing.T) {
	// belum lewat tanggal 10
	l := HitungLaporanWajib(tgl("2027-02-01"), belumUji(), tgl("2027-02-10"), 100_000)
	if l.Terlambat() || l.HariTerlambat != 0 {
		t.Errorf("tepat tanggal 10 belum terlambat: %+v", l)
	}
	if l.JatuhTempo != tgl("2027-02-10") || l.Nama() != "Februari 2027" || l.PeriodeInput() != "2027-02" {
		t.Errorf("jatuh tempo=%v nama=%q input=%q", l.JatuhTempo, l.Nama(), l.PeriodeInput())
	}
	if l.TotalKekurangan != 200_000 {
		t.Errorf("total = %d, want 200000", l.TotalKekurangan)
	}

	// 1 hari setelah jatuh tempo
	if l = HitungLaporanWajib(tgl("2027-02-01"), belumUji(), tgl("2027-02-11"), 100_000); l.HariTerlambat != 1 || !l.Terlambat() {
		t.Errorf("hari terlambat = %d, want 1", l.HariTerlambat)
	}

	// periode lama: terlambat lintas bulan
	if l = HitungLaporanWajib(tgl("2027-01-15"), nil, tgl("2027-02-11"), 100_000); l.HariTerlambat != 32 || l.TotalKekurangan != 0 {
		t.Errorf("periode Januari: hari = %d, total = %d", l.HariTerlambat, l.TotalKekurangan)
	}

	// periode di masa depan: belum jatuh tempo
	if l = HitungLaporanWajib(tgl("2027-03-01"), belumUji(), tgl("2027-02-11"), 100_000); l.Terlambat() {
		t.Error("periode masa depan tidak boleh terlambat")
	}
}

func TestWajibBelumBayar_Service(t *testing.T) {
	repo := &fakeLaporanRepo{wajibPeriode: belumUji()}
	s := svcLaporan(repo, "2027-02-11").DenganWajib(100_000)

	if _, err := s.WajibBelumBayar(context.Background(), WajibInput{}); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("tanpa login: %v", err)
	}

	ctx := ctxLogin()
	l, err := s.WajibBelumBayar(ctx, WajibInput{}) // kosong = bulan ini
	if err != nil {
		t.Fatal(err)
	}
	if repo.periodeMinta != tgl("2027-02-01") || l.HariTerlambat != 1 || l.TotalKekurangan != 200_000 {
		t.Errorf("periode=%v hari=%d total=%d", repo.periodeMinta, l.HariTerlambat, l.TotalKekurangan)
	}

	if _, err = s.WajibBelumBayar(ctx, WajibInput{Periode: "2027-01"}); err != nil || repo.periodeMinta != tgl("2027-01-01") {
		t.Errorf("periode Januari: err=%v diminta=%v", err, repo.periodeMinta)
	}

	var ve *ValidasiError
	if _, err = s.WajibBelumBayar(ctx, WajibInput{Periode: "bulan-lalu"}); !errors.As(err, &ve) || ve.Pesan["periode"] == "" {
		t.Errorf("format salah: %v", err)
	}

	repo.galat = errors.New("db mati")
	if _, err = s.WajibBelumBayar(ctx, WajibInput{}); err == nil {
		t.Error("galat repo harus diteruskan")
	}
}

func TestCSV_Wajib(t *testing.T) {
	l := HitungLaporanWajib(tgl("2027-02-01"), belumUji(), tgl("2027-02-11"), 100_000)
	b, err := l.CSV()
	out := barisCSV(t, b, err)

	want := []string{
		"Nomor anggota;Nama;Periode;Jatuh tempo;Hari terlambat;Kekurangan",
		"KKS-0002;Dewi Anggraini;2027-02;2027-02-10;1;100000",
		"KKS-0005;'=Maya;2027-02;2027-02-10;1;100000",
		"Total;2 anggota;;;;200000",
	}
	if strings.Join(out, "|") != strings.Join(want, "|") {
		t.Errorf("CSV =\n%s\nwant\n%s", strings.Join(out, "\n"), strings.Join(want, "\n"))
	}
}
