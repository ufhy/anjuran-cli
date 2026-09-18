package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UnduhRepo adalah repo tempat berkas rilis dicari.
const UnduhRepo = "ufhy/anjuran-cli"

// EnvAsalRilis mengganti asal berkas rilis. Untuk cermin, jaringan tertutup,
// dan supaya jalur ini bisa diuji tanpa menyentuh jaringan sungguhan.
const EnvAsalRilis = "ANJURAN_RELEASE_URL"

// errTakBisaUnduh berarti pengunduhan bukan jalan yang tersedia di sini —
// bukan bahwa ia dicoba lalu gagal.
var errTakBisaUnduh = errors.New("tidak ada rilis yang bisa diunduh")

// batasUnduh menjaga `anjuran up` tidak menggantung pada jaringan yang buruk.
// Arsipnya sekitar 7 MB; satu menit sudah sangat longgar.
const batasUnduh = 60 * time.Second

// unduhRilis mengambil berkas rilis untuk platform host.
//
// Ini jalan keluar bagi mayoritas pengguna, dan selama ini justru merekalah
// yang tidak punya. Yang memasang lewat `curl | sh` tidak punya pohon sumber
// maupun toolchain Go, sehingga memasang dari laptop macOS ke server Linux
// berakhir dengan pesan yang menyuruh menjalankan `make cross` di repo yang
// tidak pernah mereka unduh.
//
// Versinya diikat ke versi binary LOKAL, bukan ke rilis terbaru. Perintah ini
// memasang "anjuran yang sama seperti di mesin ini"; diam-diam memasang versi
// lain di host membuat perbedaan perilaku antara dua mesin menjadi teka-teki
// yang tidak ada petunjuknya.
func unduhRilis(p Platform, versi string) (Source, func(), error) {
	noop := func() {}

	// Binary dari pohon kerja tidak punya rilis yang bersesuaian. Pemakainya
	// juga hampir pasti sedang berada di dalam pohon sumber, dan di sana
	// pembangunan silang sudah menjadi jalan yang lebih baik.
	if versi == "" || versi == "dev" {
		return Source{}, noop, errTakBisaUnduh
	}

	// extractArchive hanya membongkar tar.gz. Host Windows tetap bisa dipasang
	// lewat --from; yang tidak boleh terjadi adalah mengunduh 7 MB lalu baru
	// mengaku tidak bisa membongkarnya.
	if p.OS == "windows" {
		return Source{}, noop, errTakBisaUnduh
	}

	polos := strings.TrimPrefix(versi, "v")
	nama := fmt.Sprintf("anjuran_%s_%s_%s.tar.gz", polos, p.OS, p.Arch)
	asal := os.Getenv(EnvAsalRilis)
	if asal == "" {
		asal = "https://github.com/" + UnduhRepo + "/releases/download"
	}
	tag := versi
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	dir, err := os.MkdirTemp("", "anjuran-unduh-")
	if err != nil {
		return Source{}, noop, err
	}
	bersihkan := func() { os.RemoveAll(dir) }

	arsip := filepath.Join(dir, nama)
	if err := ambil(asal+"/"+tag+"/"+nama, arsip); err != nil {
		bersihkan()
		return Source{}, noop, fmt.Errorf("%w: %v", errTakBisaUnduh, err)
	}

	// Arsipnya datang lewat jaringan dan isinya akan dijalankan di mesin
	// ORANG LAIN. Memeriksanya bukan kemewahan.
	daftar := filepath.Join(dir, "checksums.txt")
	if err := ambil(asal+"/"+tag+"/checksums.txt", daftar); err == nil {
		if err := periksaChecksum(arsip, daftar); err != nil {
			bersihkan()
			return Source{}, noop, err
		}
	}

	src, bersihkanArsip, err := extractArchive(arsip, p)
	if err != nil {
		bersihkan()
		return Source{}, noop, fmt.Errorf("%w: %v", errTakBisaUnduh, err)
	}
	src.Origin = "rilis " + tag
	return src, func() { bersihkanArsip(); bersihkan() }, nil
}

// ambil mengunduh satu berkas.
func ambil(url, tujuan string) error {
	c := &http.Client{Timeout: batasUnduh}
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}

	f, err := os.Create(tujuan)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// periksaChecksum membandingkan arsip dengan baris yang bersesuaian di
// checksums.txt.
func periksaChecksum(arsip, daftar string) error {
	isi, err := os.ReadFile(daftar)
	if err != nil {
		return err
	}
	nama := filepath.Base(arsip)

	mau := ""
	for _, baris := range strings.Split(string(isi), "\n") {
		bagian := strings.Fields(baris)
		if len(bagian) == 2 && strings.TrimPrefix(bagian[1], "*") == nama {
			mau = bagian[0]
			break
		}
	}
	if mau == "" {
		return fmt.Errorf("%s tidak ada di checksums.txt", nama)
	}

	f, err := os.Open(arsip)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if dapat := hex.EncodeToString(h.Sum(nil)); dapat != mau {
		return fmt.Errorf("checksum tidak cocok untuk %s:\n  mau   : %s\n  dapat : %s", nama, mau, dapat)
	}
	return nil
}
