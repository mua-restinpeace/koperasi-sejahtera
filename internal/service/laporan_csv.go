package service

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"time"
)

// Ekspor CSV untuk Excel berbahasa Indonesia: pemisah titik koma dan BOM UTF-8 di
// awal supaya huruf dan kolom terbuka rapi. Nominal ditulis sebagai angka polos
// (tanpa "Rp" dan titik ribuan) supaya bisa dijumlah di spreadsheet; tanggal
// ditulis YYYY-MM-DD.

func angka(n int64) string { return strconv.FormatInt(n, 10) }

func tanggalCSV(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// amanCSV mencegah "CSV injection": teks yang diawali = + - @ akan dibaca Excel
// sebagai rumus, jadi diberi tanda petik di depannya.
func amanCSV(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func tulisCSV(baris [][]string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(&buf)
	w.Comma = ';'
	if err := w.WriteAll(baris); err != nil { // WriteAll sudah memanggil Flush
		return nil, err
	}
	return buf.Bytes(), nil
}

// CSV L-3: satu baris per bulan, ditutup baris total.
func (l LaporanPendapatan) CSV() ([]byte, error) {
	baris := [][]string{{"Bulan", "Jasa", "Denda", "Biaya admin", "Total"}}
	for _, b := range l.Bulan {
		baris = append(baris, []string{b.Nama(), angka(b.Jasa), angka(b.Denda), angka(b.BiayaAdmin), angka(b.Total())})
	}
	t := l.Total
	baris = append(baris, []string{"Total", angka(t.Jasa), angka(t.Denda), angka(t.BiayaAdmin), angka(t.Total())})
	return tulisCSV(baris)
}

// CSV L-2: satu baris per angsuran yang menunggak, ditutup baris total.
func (l LaporanTunggakan) CSV() ([]byte, error) {
	baris := [][]string{{"Nomor anggota", "Nama", "Pinjaman", "Angsuran ke", "Jatuh tempo", "Hari terlambat", "Sisa jasa", "Sisa pokok", "Denda", "Total"}}
	for _, b := range l.Baris {
		baris = append(baris, []string{
			amanCSV(b.NomorAnggota), amanCSV(b.NamaAnggota), strconv.FormatInt(b.IDPinjaman, 10), strconv.Itoa(b.Urutan),
			tanggalCSV(b.JatuhTempo), strconv.Itoa(b.HariTerlambat),
			angka(b.SisaJasa), angka(b.SisaPokok), angka(b.DendaBerjalan), angka(b.Total()),
		})
	}
	baris = append(baris, []string{"Total", "", "", "", "", "", "", "", angka(l.TotalDenda), angka(l.TotalSemua())})
	return tulisCSV(baris)
}

// CSV L-1: induk pinjaman, jadwal dengan status, lalu riwayat pembayaran.
func (k KartuPinjaman) CSV() ([]byte, error) {
	p := k.Pinjaman
	baris := [][]string{
		{"Kartu pinjaman", "#" + strconv.FormatInt(p.ID, 10)},
		{"Anggota", amanCSV(p.NomorAnggota), amanCSV(p.NamaAnggota)},
		{"Jumlah pinjaman", angka(p.Jumlah)},
		{"Tenor (bulan)", strconv.Itoa(p.TenorBulan)},
		{"Tanggal pencairan", tanggalCSV(p.TanggalPencairan)},
		{"Posisi per tanggal", tanggalCSV(k.Tanggal)},
		{},
		{"Angsuran ke", "Jatuh tempo", "Tagihan pokok", "Tagihan jasa", "Pokok dibayar", "Jasa dibayar", "Denda dibayar", "Sisa pokok", "Sisa jasa", "Denda terutang", "Tanggal lunas", "Status"},
	}
	for _, b := range k.Baris {
		baris = append(baris, []string{
			strconv.Itoa(b.Urutan), tanggalCSV(b.JatuhTempo), angka(b.TagihanPokok), angka(b.TagihanJasa),
			angka(b.PokokDibayar), angka(b.JasaDibayar), angka(b.DendaDibayar),
			angka(b.SisaPokok), angka(b.SisaJasa), angka(b.DendaTerutang), tanggalCSV(b.LunasPada), b.Status,
		})
	}
	baris = append(baris,
		[]string{"Sisa pokok", angka(k.SisaPokok)},
		[]string{"Sisa jasa", angka(k.SisaJasa)},
		[]string{"Denda terutang", angka(k.DendaTerutang)},
		[]string{"Total sisa tagihan", angka(k.SisaTagihan())},
		[]string{},
		[]string{"Tanggal bayar", "Jenis", "Jumlah diterima", "Biaya administrasi"},
	)
	for _, b := range k.Pembayaran {
		baris = append(baris, []string{tanggalCSV(b.TanggalBayar), string(b.Jenis), angka(b.Jumlah), angka(b.BiayaAdmin)})
	}
	return tulisCSV(baris)
}
