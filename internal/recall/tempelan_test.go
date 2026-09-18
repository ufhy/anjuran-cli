package recall

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Penanda tempelan harus melewati BATAS PROSES.
//
// Setiap penekanan tombol pemicu menumbuhkan proses anjuran yang baru, dan
// satu baris yang ditempel memuat banyak pemicu. Yang mengetahui adanya
// tempelan sudah keluar sebelum pemicu berikutnya dijalankan, jadi ingatan di
// dalam memori tidak akan pernah terpakai.
func TestPenandaTempelanBertahanAntarProses(t *testing.T) {
	t.Setenv(EnvCacheDir, t.TempDir())

	if SedangMenempel() {
		t.Error("belum ada tempelan, tetapi sudah dianggap sedang menempel")
	}
	TandaiTempelan()
	if !SedangMenempel() {
		t.Error("penanda ditulis tetapi tidak terbaca")
	}
}

// Penanda yang sudah lewat tidak boleh mematikan completion selamanya.
func TestPenandaTempelanKedaluwarsa(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvCacheDir, dir)
	TandaiTempelan()

	lampau := time.Now().Add(-2 * jendelaTempelan)
	if err := os.Chtimes(berkasTempelan(), lampau, lampau); err != nil {
		t.Fatal(err)
	}
	if SedangMenempel() {
		t.Error("penanda lama masih dianggap berlaku")
	}
}

// Jam sistem yang disetel mundur meninggalkan berkas bertanggal masa depan.
// Itu tidak boleh membuat completion mati sampai jamnya menyusul.
func TestPenandaTempelanDariMasaDepan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvCacheDir, dir)
	TandaiTempelan()

	depan := time.Now().Add(time.Hour)
	if err := os.Chtimes(berkasTempelan(), depan, depan); err != nil {
		t.Fatal(err)
	}
	if SedangMenempel() {
		t.Error("penanda bertanggal masa depan masih dianggap berlaku")
	}
}

// Tanpa direktori cache yang bisa dipakai, penjagaan ini diam saja — bukan
// gagal. Kehilangannya hanya membuat sebuah kotak muncul.
func TestTanpaCacheTidakMengganggu(t *testing.T) {
	// Sebuah BERKAS dipakai sebagai lokasi cache: MkdirAll menolaknya, jadi
	// CacheDir mengembalikan kosong — keadaan yang sama dengan direktori rumah
	// yang hanya-baca, tanpa perlu mengandalkan nama yang tidak sah dan
	// berbeda aturannya antar sistem.
	berkas := filepath.Join(t.TempDir(), "bukan-direktori")
	if err := os.WriteFile(berkas, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvCacheDir, berkas)

	TandaiTempelan()
	if SedangMenempel() {
		t.Error("tanpa cache seharusnya menjawab tidak sedang menempel")
	}
}
