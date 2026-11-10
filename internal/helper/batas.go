package helper

// menghitung batas pinjaman anggota
func HitungBatas(totalPokok, totalWajib int64, kelipatan int) int64 {
	return int64(kelipatan) * (totalPokok + totalWajib)
}
