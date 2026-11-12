# Spesifikasi Perhitungan: Koperasi Karyawan Sejahtera

Status: draft v1. Every number in section 6 is meant to become a table-driven test.

Conventions
- All money is `int64` rupiah. No floats anywhere in the calculation core.
- All dates are civil dates (Asia/Jakarta), never timestamps.
- Parameters come from app config, not the schema:
  `SimpananPokok=500000`, `SimpananWajib=100000`, `JasaBps=100` (1%/bulan flat),
  `DendaPerHari=2000`, `BiayaAdminBps=200` (2%), `KelipatanBatas=3`, tenor 6..24.
  `jasa_per_bulan_bps` and `denda_per_hari` are copied onto each `pinjaman` row at disbursement.
- The calculation core is pure: no DB, no clock. "Today" is always passed in as `asOf`.

---

## 1. Batas pinjaman

```
batas = 3 x (SUM setoran pokok + SUM setoran wajib)   // non-voided rows only
```

A loan is allowed only if all of these hold:
- member status is `aktif`
- the member has no loan with status `aktif`
- `jumlah <= batas`
- `6 <= tenor <= 24`

## 2. Jadwal angsuran

```
jasa_total   = jumlah x jasa_bps x tenor / 10000      (integer division)
pokok_i      = jumlah / tenor                         (i = 1..tenor-1)
pokok_akhir  = jumlah - pokok_i x (tenor-1)           (last installment absorbs remainder)
jasa_i       = jasa_total / tenor                     (i = 1..tenor-1)
jasa_akhir   = jasa_total - jasa_i x (tenor-1)
jatuh_tempo_k = tanggal 10 of the month (disbursement month + k)
```

Notes:
- The first installment is always the 10th of the month after disbursement, even if the
  loan is disbursed late in the month (disbursed on 28 Nov, first due 10 Dec).
- `urutan` 1..tenor. December rolls over into January of the next year.
- For round loan amounts (multiples of Rp100) jasa divides exactly.

## 3. Denda

Per installment:

```
tanggal_akhir  = tanggal the installment's jasa AND pokok became fully paid,
                 or asOf if still unpaid
hari_terlambat = max(0, tanggal_akhir - jatuh_tempo)     // in days
denda_total    = DendaPerHari x hari_terlambat
denda_terutang = denda_total - SUM(denda_dibayar)
```

- Paying on the 10th costs nothing. The 11th is 1 day late (Rp2.000).
- Denda keeps accruing until jasa and pokok are fully paid, even if the member
  has already paid some denda. Denda is not capped.
- Once an installment is fully paid, its `denda_total` is frozen.
- Denda is computed, never stored as a balance; only `denda_dibayar` is stored.

## 4. Alokasi pembayaran

Input: all non-voided payments of a loan, sorted by `(tanggal_bayar, id)`.
Allocation is a pure function of that list, replayed from scratch.

For each payment, walk installments in `urutan` order, skipping fully settled ones.
For each installment, as of the payment's `tanggal_bayar`:

1. pay `denda_terutang`
2. pay unpaid `jasa`
3. pay unpaid `pokok`

If money is left, continue to the next installment (this includes installments not
yet due, whose denda is 0). Each installment yields one `alokasi_pembayaran` row.

Rules:
- A payment larger than the total outstanding as of its date is rejected. Early payoff
  is a separate action (section 5). Change is handled by the pengurus entering only the
  amount actually kept.
- Voiding or backdating a payment triggers a full replay of the loan's allocations.
- A payment smaller than the denda owed goes entirely to denda; jasa and pokok are untouched.

## 5. Pelunasan dipercepat

As of `asOf` (the payoff date):

```
sisa_pokok         = SUM unpaid pokok over all installments
denda              = SUM denda_terutang over all installments, accrued through asOf
jasa_jatuh_tempo   = SUM unpaid jasa of installments with jatuh_tempo <= asOf
jasa_bulan_berjalan= unpaid jasa of the FIRST installment with jatuh_tempo > asOf
                     whose jasa is not yet fully paid (just that one month)
biaya_admin        = sisa_pokok x 200 / 10000      (integer division)

total_pelunasan = sisa_pokok + denda + jasa_jatuh_tempo
                + jasa_bulan_berjalan + biaya_admin
```

After payoff:
- `pinjaman.status = 'lunas'`, `tanggal_lunas = asOf`.
- Jasa of later installments is not charged (derivable as `jasa_tagihan - jasa_dibayar`).
- `biaya_administrasi` is stored on the payment and counted as income.

Assumption to confirm: "jasa satu bulan berjalan" means the next installment's jasa that
is not yet due. If that month was already prepaid, it moves to the one after.

---

## 6. Worked examples (these become the tests)

### A. Schedule with remainder

Loan Rp10.000.000, tenor 7, disbursed 2026-11-05, jasa 1%.

| Urutan | Jatuh tempo | Pokok     | Jasa    | Angsuran  |
|--------|-------------|-----------|---------|-----------|
| 1      | 2026-12-10  | 1.428.571 | 100.000 | 1.528.571 |
| 2      | 2027-01-10  | 1.428.571 | 100.000 | 1.528.571 |
| 3      | 2027-02-10  | 1.428.571 | 100.000 | 1.528.571 |
| 4      | 2027-03-10  | 1.428.571 | 100.000 | 1.528.571 |
| 5      | 2027-04-10  | 1.428.571 | 100.000 | 1.528.571 |
| 6      | 2027-05-10  | 1.428.571 | 100.000 | 1.528.571 |
| 7      | 2027-06-10  | 1.428.574 | 100.000 | 1.528.574 |

Checks: sum of pokok = 10.000.000, total jasa = 700.000, total = 10.700.000.

### B. Batas pinjaman

Member with pokok Rp500.000 + 20 months of wajib (Rp2.000.000) = Rp2.500.000.
`batas` = Rp7.500.000. A loan of Rp7.500.000 is accepted, Rp7.500.001 is rejected.

### C. Partial payments across overdue installments, then payoff

Loan Rp6.000.000, tenor 6, disbursed 2026-11-05. Every installment: pokok 1.000.000,
jasa 60.000. Due dates: 2026-12-10, 2027-01-10, 2027-02-10, 2027-03-10, 2027-04-10, 2027-05-10.

**Payment 1: Rp1.500.000 on 2027-01-15** (installment 1 is 36 days late, installment 2 is 5 days late)

| Installment | Denda  | Jasa   | Pokok     |
|-------------|--------|--------|-----------|
| 1           | 72.000 | 60.000 | 1.000.000 |
| 2           | 10.000 | 60.000 | 298.000   |

Totals: denda 82.000 + jasa 120.000 + pokok 1.298.000 = 1.500.000. Installment 1 is cleared
on 2027-01-15, so its denda is frozen at 72.000.

**Payment 2: Rp800.000 on 2027-02-12** (installment 2 is now 33 days late, installment 3 is 2 days late)

| Installment | Denda  | Jasa   | Pokok   |
|-------------|--------|--------|---------|
| 2           | 56.000 | 0      | 702.000 |
| 3           | 4.000  | 38.000 | 0       |

Installment 2 total denda is 66.000 (33 days); 10.000 was already paid, so 56.000 is owed.
Totals: denda 60.000 + jasa 38.000 + pokok 702.000 = 800.000.
After this: sisa pokok = 4.000.000; installment 3 still owes jasa 22.000 and pokok 1.000.000.

**Arrears list as of 2027-02-11** (before payment 2):
- Installment 2: pokok 702.000, jasa 0, denda 54.000 (32 days = 64.000, minus 10.000 paid)
- Installment 3: pokok 1.000.000, jasa 60.000, denda 2.000 (1 day)

**Early payoff on 2027-02-20** (after both payments):

| Component                                   | Amount    |
|---------------------------------------------|-----------|
| Sisa pokok                                  | 4.000.000 |
| Denda installment 3 (10 days = 20.000, minus 4.000 paid) | 16.000 |
| Jasa installment 3 (due, unpaid)            | 22.000    |
| Jasa bulan berjalan (installment 4, due 03-10) | 60.000 |
| Biaya admin 2% x 4.000.000                  | 80.000    |
| **Total pelunasan**                         | **4.178.000** |

Jasa of installments 5 and 6 (120.000) is not charged.

**Monthly income report (cash basis, by `tanggal_bayar`)**

| Month   | Jasa    | Denda   | Biaya admin |
|---------|---------|---------|-------------|
| 2027-01 | 120.000 | 82.000  | 0           |
| 2027-02 | 120.000 | 76.000  | 80.000      |

Feb jasa = 38.000 (payment 2) + 22.000 + 60.000 (payoff). Feb denda = 60.000 + 16.000.

### D. Void and replay

Void payment 1 from example C. Payment 2 (Rp800.000 on 2027-02-12) is replayed alone:
installment 1 is 64 days late, so denda 128.000 + jasa 60.000 + pokok 612.000 = 800.000,
and installment 1 is left with 388.000 pokok unpaid.

### E. Edge cases

| Case | Expected |
|------|----------|
| Pay on the 10th (due date) | 0 denda |
| Pay on the 11th | 2.000 denda |
| Payment smaller than denda owed | all to denda, jasa and pokok untouched |
| Payment exceeds total outstanding | rejected |
| Disbursed 2026-12-28 | first due 2027-01-10 |
| Pay installment before it is due | applied to jasa then pokok, denda 0 |
| Payment backdated before an existing payment | later payments are re-allocated |
| Member with an active loan applies again | rejected |
| Member exit with an active loan | rejected |

---

## 7. API inti perhitungan (package `internal/service`)

Fungsi murni: tanpa basis data dan tanpa membaca jam. Nama domain Bahasa Indonesia, nama teknis Bahasa Inggris.

```go
func HitungBatas(totalPokok, totalWajib int64, kelipatan int) int64
func BuatJadwal(jumlah int64, tenor, jasaBps int, pencairan time.Time) ([]model.Angsuran, error)
func HitungDenda(jatuhTempo, tanggalAkhir time.Time, dendaPerHari int64) int64

// Memutar ulang seluruh pembayaran (yang belum dibatalkan) menurut (tanggal bayar, ID).
func HitungAlokasi(jadwal []model.Angsuran, pembayaran []model.Pembayaran, aturan AturanHitung) ([]model.Alokasi, error)

// Keadaan tiap angsuran per tanggal asOf: sisa pokok, sisa jasa, denda terutang, tanggal lunas.
// Dasar untuk kartu pinjaman (L-1) dan daftar tunggakan (L-2).
func HitungStatus(jadwal []model.Angsuran, pembayaran []model.Pembayaran, asOf time.Time, aturan AturanHitung) ([]model.StatusAngsuran, error)

// Rincian pelunasan dipercepat; status berasal dari HitungStatus dengan asOf yang sama.
func HitungPelunasan(status []model.StatusAngsuran, asOf time.Time, biayaAdminBps int) model.RincianPelunasan
```

Kesalahan yang dapat dikenali dengan `errors.Is`: `ErrPembayaranBerlebih`, `ErrPinjamanSudahLunas`,
`ErrJumlahPembayaranTidakValid`, `ErrBiayaAdminTidakValid`, `ErrJumlahPelunasanTidakSesuai`, `ErrJenisTidakDikenal`.

Catatan penerapan:
- Pembayaran pelunasan harus tepat sama dengan `HitungPelunasan` pada tanggalnya, kalau tidak ditolak.
- Setelah pelunasan, jasa bulan-bulan berikutnya tercatat di `JasaDihapus` dan seluruh angsuran berstatus lunas.
- Contoh A sampai E pada dokumen ini dites di `internal/service/*_test.go`. Selain itu ada tes acak
  (`alokasi_properti_test.go`) yang memeriksa sifat umum: uang masuk sama dengan uang teralokasi, urutan
  masukan tidak berpengaruh, angsuran terlama lunas lebih dulu, pembatalan pembayaran apa pun selalu
  dapat dihitung ulang, dan pelunasan tidak pernah lebih mahal dari tagihan penuh ditambah administrasi.

## 8. Open points

1. Denda stops accruing when jasa and pokok are fully paid, not when denda is paid (section 3).
2. Reading of "jasa satu bulan berjalan" in payoff (section 5).
3. Overpayment beyond the outstanding balance is rejected rather than refunded or credited.
4. Rounding for non-round loan amounts follows section 2; confirm with the pengurus how
   they round today.
