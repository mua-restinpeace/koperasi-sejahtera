package helper

import (
	"errors"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

func tgl(tahun int, bulan time.Month, hari int) time.Time {
	return time.Date(tahun, bulan, hari, 0, 0, 0, 0, time.UTC)
}

func ang(urutan int, jatuhTempo time.Time, pokok, jasa int64) model.Angsuran {
	return model.Angsuran{Urutan: urutan, JatuhTempo: jatuhTempo, Pokok: pokok, Jasa: jasa}
}

// Contoh A pada docs/spec-perhitungan.md: Rp10.000.000, tenor 7, dicairkan 2026-11-05.
func TestBuatJadwal_ContohA(t *testing.T) {
	jadwal, err := BuatJadwal(10_000_000, 7, 100, tgl(2026, time.November, 5))
	if err != nil {
		t.Fatalf("BuatJadwal error: %v", err)
	}
	want := []model.Angsuran{
		ang(1, tgl(2026, time.December, 10), 1_428_571, 100_000),
		ang(2, tgl(2027, time.January, 10), 1_428_571, 100_000),
		ang(3, tgl(2027, time.February, 10), 1_428_571, 100_000),
		ang(4, tgl(2027, time.March, 10), 1_428_571, 100_000),
		ang(5, tgl(2027, time.April, 10), 1_428_571, 100_000),
		ang(6, tgl(2027, time.May, 10), 1_428_571, 100_000),
		ang(7, tgl(2027, time.June, 10), 1_428_574, 100_000),
	}
	if len(jadwal) != len(want) {
		t.Fatalf("jumlah angsuran = %d, want %d", len(jadwal), len(want))
	}
	for i := range want {
		if !jadwal[i].JatuhTempo.Equal(want[i].JatuhTempo) || jadwal[i].Urutan != want[i].Urutan ||
			jadwal[i].Pokok != want[i].Pokok || jadwal[i].Jasa != want[i].Jasa {
			t.Errorf("angsuran %d = %+v, want %+v", i+1, jadwal[i], want[i])
		}
	}
}

func TestBuatJadwal_Tabel(t *testing.T) {
	tests := []struct {
		nama       string
		jumlah     int64
		tenor      int
		jasaBps    int
		pencairan  time.Time
		pertamaTmp time.Time
		akhirTmp   time.Time
		pokokAkhir int64
		jasaAkhir  int64
	}{
		{
			nama:   "Rp6.000.000 tenor 6 habis dibagi",
			jumlah: 6_000_000, tenor: 6, jasaBps: 100, pencairan: tgl(2026, time.November, 5),
			pertamaTmp: tgl(2026, time.December, 10), akhirTmp: tgl(2027, time.May, 10),
			pokokAkhir: 1_000_000, jasaAkhir: 60_000,
		},
		{
			nama:   "pencairan akhir bulan tetap jatuh tempo tanggal 10 bulan berikutnya",
			jumlah: 6_000_000, tenor: 6, jasaBps: 100, pencairan: tgl(2026, time.December, 28),
			pertamaTmp: tgl(2027, time.January, 10), akhirTmp: tgl(2027, time.June, 10),
			pokokAkhir: 1_000_000, jasaAkhir: 60_000,
		},
		{
			nama:   "sisa pembulatan jasa masuk angsuran terakhir",
			jumlah: 1_000_050, tenor: 6, jasaBps: 100, pencairan: tgl(2026, time.November, 5),
			pertamaTmp: tgl(2026, time.December, 10), akhirTmp: tgl(2027, time.May, 10),
			pokokAkhir: 166_675, jasaAkhir: 10_003, // jasa total 60.003 = 5 x 10.000 + 10.003
		},
		{
			nama:   "tenor 24 melewati dua tahun",
			jumlah: 24_000_000, tenor: 24, jasaBps: 100, pencairan: tgl(2026, time.January, 31),
			pertamaTmp: tgl(2026, time.February, 10), akhirTmp: tgl(2028, time.January, 10),
			pokokAkhir: 1_000_000, jasaAkhir: 240_000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			jadwal, err := BuatJadwal(tt.jumlah, tt.tenor, tt.jasaBps, tt.pencairan)
			if err != nil {
				t.Fatalf("BuatJadwal error: %v", err)
			}
			if len(jadwal) != tt.tenor {
				t.Fatalf("jumlah angsuran = %d, want %d", len(jadwal), tt.tenor)
			}
			if got := jadwal[0].JatuhTempo; !got.Equal(tt.pertamaTmp) {
				t.Errorf("jatuh tempo pertama = %s, want %s", got.Format("2006-01-02"), tt.pertamaTmp.Format("2006-01-02"))
			}
			akhir := jadwal[len(jadwal)-1]
			if !akhir.JatuhTempo.Equal(tt.akhirTmp) {
				t.Errorf("jatuh tempo terakhir = %s, want %s", akhir.JatuhTempo.Format("2006-01-02"), tt.akhirTmp.Format("2006-01-02"))
			}
			if akhir.Pokok != tt.pokokAkhir || akhir.Jasa != tt.jasaAkhir {
				t.Errorf("angsuran terakhir pokok/jasa = %d/%d, want %d/%d", akhir.Pokok, akhir.Jasa, tt.pokokAkhir, tt.jasaAkhir)
			}

			// Invarian: pokok seluruh angsuran = jumlah pinjaman; jasa seluruhnya = jasa flat total.
			var sumPokok, sumJasa int64
			for i, a := range jadwal {
				sumPokok += a.Pokok
				sumJasa += a.Jasa
				if a.Urutan != i+1 {
					t.Errorf("urutan ke-%d = %d", i+1, a.Urutan)
				}
				if a.JatuhTempo.Day() != TanggalJatuhTempo {
					t.Errorf("angsuran %d jatuh tempo tanggal %d, want 10", a.Urutan, a.JatuhTempo.Day())
				}
			}
			if sumPokok != tt.jumlah {
				t.Errorf("total pokok = %d, want %d", sumPokok, tt.jumlah)
			}
			wantJasa := tt.jumlah * int64(tt.jasaBps) * int64(tt.tenor) / 10000
			if sumJasa != wantJasa {
				t.Errorf("total jasa = %d, want %d", sumJasa, wantJasa)
			}
		})
	}
}

func TestBuatJadwal_MasukanTidakValid(t *testing.T) {
	tests := []struct {
		nama    string
		jumlah  int64
		tenor   int
		jasaBps int
		want    error
	}{
		{"jumlah nol", 0, 6, 100, ErrJumlahTidakValid},
		{"jumlah negatif", -1, 6, 100, ErrJumlahTidakValid},
		{"tenor nol", 1_000_000, 0, 100, ErrTenorTidakValid},
		{"jasa negatif", 1_000_000, 6, -1, ErrJasaTidakValid},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			if _, err := BuatJadwal(tt.jumlah, tt.tenor, tt.jasaBps, tgl(2026, time.November, 5)); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
