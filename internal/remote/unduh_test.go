package remote

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// rilisPalsu menyajikan satu arsip rilis beserta checksums.txt-nya.
func rilisPalsu(t *testing.T, nama string, rusakkanChecksum bool) string {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	tulis := func(path, isi string, mode int64) {
		tw.WriteHeader(&tar.Header{Name: path, Mode: mode, Size: int64(len(isi)), Typeflag: tar.TypeReg})
		tw.Write([]byte(isi))
	}
	tulis("anjuran", "biner", 0o755)
	tulis("specs/git.json", "{}", 0o644)
	tulis("extra/cd.json", "{}", 0o644)
	tw.Close()
	zw.Close()
	arsip := buf.Bytes()

	jumlah := sha256.Sum256(arsip)
	hexJumlah := hex.EncodeToString(jumlah[:])
	if rusakkanChecksum {
		hexJumlah = strings.Repeat("0", 64)
	}
	checksums := fmt.Sprintf("%s  %s\n", hexJumlah, nama)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, nama):
			w.Write(arsip)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Write([]byte(checksums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// Yang memasang lewat `curl | sh` tidak punya pohon sumber maupun toolchain
// Go. Tanpa jalur ini, memasang dari laptop macOS ke server Linux berakhir
// dengan pesan yang menyuruh menjalankan `make cross` di repo yang tidak
// pernah mereka unduh.
func TestUnduhRilisMemakaiBerkasRilis(t *testing.T) {
	nama := "anjuran_1.2.3_linux_arm64.tar.gz"
	t.Setenv(EnvAsalRilis, rilisPalsu(t, nama, false))

	src, bersihkan, err := unduhRilis(Platform{OS: "linux", Arch: "arm64"}, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	defer bersihkan()

	if src.Binary == "" {
		t.Error("binary tidak ditemukan di dalam arsip")
	}
	// Tambalan tulisan tangan harus ikut. Di situlah cd, ssh, docker, dan
	// kubectl — dan justru di host remote perintahnya paling tidak dihafal.
	if src.Extra == "" {
		t.Error("extra/ tidak ikut terbawa dari arsip rilis")
	}
	if src.Specs == "" {
		t.Error("specs/ tidak ikut terbawa dari arsip rilis")
	}
	if !strings.Contains(src.Origin, "v1.2.3") {
		t.Errorf("Origin = %q, seharusnya menyebut versinya", src.Origin)
	}
}

// Versi tanpa awalan v tetap menemukan tagnya.
func TestUnduhRilisMenambahkanAwalanV(t *testing.T) {
	nama := "anjuran_1.2.3_linux_amd64.tar.gz"
	t.Setenv(EnvAsalRilis, rilisPalsu(t, nama, false))

	_, bersihkan, err := unduhRilis(Platform{OS: "linux", Arch: "amd64"}, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	bersihkan()
}

// Checksum yang tidak cocok MEMBATALKAN pemasangan, bukan jatuh diam-diam ke
// jalan lain. Isinya akan dijalankan di mesin orang lain; "coba cara lain"
// adalah jawaban yang salah untuk berkas yang tidak sesuai harapan.
func TestUnduhRilisMenolakChecksumSalah(t *testing.T) {
	nama := "anjuran_1.2.3_linux_arm64.tar.gz"
	t.Setenv(EnvAsalRilis, rilisPalsu(t, nama, true))

	_, bersihkan, err := unduhRilis(Platform{OS: "linux", Arch: "arm64"}, "v1.2.3")
	bersihkan()
	if err == nil {
		t.Fatal("mau error untuk checksum yang tidak cocok")
	}
	if errors.Is(err, errTakBisaUnduh) {
		t.Error("checksum salah tidak boleh diperlakukan sebagai 'tidak ada rilis'")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("pesan harus menyebut checksum: %v", err)
	}
}

// Binary dari pohon kerja tidak punya rilis yang bersesuaian.
func TestUnduhRilisMelewatiVersiDev(t *testing.T) {
	t.Setenv(EnvAsalRilis, "http://127.0.0.1:1") // tidak boleh dihubungi sama sekali
	for _, versi := range []string{"", "dev"} {
		_, bersihkan, err := unduhRilis(Platform{OS: "linux", Arch: "arm64"}, versi)
		bersihkan()
		if !errors.Is(err, errTakBisaUnduh) {
			t.Errorf("versi %q: err = %v, mau errTakBisaUnduh", versi, err)
		}
	}
}

// Host Windows dilewati SEBELUM mengunduh: arsipnya zip, dan membongkarnya
// belum didukung. Mengunduh 7 MB lalu baru mengaku tidak bisa membongkarnya
// adalah pemborosan yang bisa dihindari dengan satu pemeriksaan.
func TestUnduhRilisMelewatiWindows(t *testing.T) {
	t.Setenv(EnvAsalRilis, "http://127.0.0.1:1")
	_, bersihkan, err := unduhRilis(Platform{OS: "windows", Arch: "amd64"}, "v1.2.3")
	bersihkan()
	if !errors.Is(err, errTakBisaUnduh) {
		t.Errorf("err = %v, mau errTakBisaUnduh", err)
	}
}

// Rilis yang tidak punya berkas untuk platform itu tidak menghentikan apa pun;
// jalan lain masih ada sesudahnya.
func TestUnduhRilisTidakAdaBerkasnya(t *testing.T) {
	t.Setenv(EnvAsalRilis, rilisPalsu(t, "anjuran_1.2.3_linux_arm64.tar.gz", false))

	_, bersihkan, err := unduhRilis(Platform{OS: "linux", Arch: "riscv64"}, "v1.2.3")
	bersihkan()
	if !errors.Is(err, errTakBisaUnduh) {
		t.Errorf("err = %v, mau errTakBisaUnduh", err)
	}
}

// Arsip rilis dipakai lebih dulu daripada pembangunan silang, dan itu harus
// terlihat dari jalur yang sebenarnya — bukan hanya dari fungsi di bawahnya.
func TestResolveSourceMemakaiRilisSebelumMembangun(t *testing.T) {
	nama := "anjuran_9.9.9_linux_arm64.tar.gz"
	t.Setenv(EnvAsalRilis, rilisPalsu(t, nama, false))
	// Direktori kerja di dalam uji ini adalah pohon sumber anjuran, jadi
	// pembangunan silang SEBENARNYA mungkin. Origin membuktikan mana yang
	// dipilih.
	if _, err := os.Stat("../../go.mod"); err != nil {
		t.Skip("bukan pohon sumber")
	}

	src, bersihkan, err := ResolveSource("", Platform{OS: "linux", Arch: "arm64"}, nil, nil, "9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	defer bersihkan()
	if !strings.Contains(src.Origin, "rilis") {
		t.Errorf("Origin = %q, mau berasal dari rilis", src.Origin)
	}
}
