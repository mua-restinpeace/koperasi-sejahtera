package helper

import "testing"

// Contoh B pada docs/spec-perhitungan.md: pokok Rp500.000 + wajib 20 bulan (Rp2.000.000).
func TestHitungBatas(t *testing.T) {
	tests := []struct {
		nama       string
		pokok      int64
		wajib      int64
		kelipatan  int
		wantBatas  int64
		jumlahUji  int64
		wantDiizin bool
	}{
		{"contoh B, tepat di batas", 500_000, 2_000_000, 3, 7_500_000, 7_500_000, true},
		{"contoh B, satu rupiah di atas batas", 500_000, 2_000_000, 3, 7_500_000, 7_500_001, false},
		{"anggota baru, hanya pokok", 500_000, 0, 3, 1_500_000, 1_500_000, true},
		{"belum membayar apa pun", 0, 0, 3, 0, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			got := HitungBatas(tt.pokok, tt.wajib, tt.kelipatan)
			if got != tt.wantBatas {
				t.Fatalf("HitungBatas = %d, want %d", got, tt.wantBatas)
			}
			if diizinkan := tt.jumlahUji <= got; diizinkan != tt.wantDiizin {
				t.Errorf("jumlah %d diizinkan = %v, want %v", tt.jumlahUji, diizinkan, tt.wantDiizin)
			}
		})
	}
}
