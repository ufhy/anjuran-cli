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

// Batas waktu dipisahkan menurut yang ditunggu.
//
// http.Client.Timeout mencakup SELURUH permintaan, termasuk membaca isinya.
// Satu angka untuk keduanya berarti memilih antara menggantung lama pada
// pertanyaan kecil, atau memutus unduhan 7 MB di tengah jalan pada sambungan
// yang lambat — dan yang kedua sudah pernah terjadi.
const (
	// batasTanya untuk pertanyaan kecil ke GitHub API: beberapa kilobyte.
	batasTanya = 30 * time.Second
	// batasUnduh untuk arsip rilis. Longgar dengan sengaja: lebih baik lambat
	// daripada gagal, karena kegagalannya berarti pemasangan yang batal.
	batasUnduh = 10 * time.Minute
)

// UnduhRilis mengunduh berkas rilis sebuah versi untuk sebuah platform.
//
// Dipakai `anjuran update`, yang berbeda dari jalur `up`: di sana versinya
// ditentukan pemanggil, bukan diikat ke versi lokal.
func UnduhRilis(p Platform, versi string) (Source, func(), error) {
	return unduhRilis(p, versi)
}

// RilisTerbaru menanyakan tag rilis terakhir kepada GitHub.
//
// /releases/latest sengaja TIDAK memuat prarilis — itulah gunanya. Tetapi
// selama proyek ini belum punya rilis stabil, satu-satunya yang ada adalah
// beta, dan berkeras pada "latest" berarti berkata tidak ada apa-apa padahal
// berkasnya ada. Nilai kedua menandai bahwa yang ditemukan sebuah prarilis,
// supaya pemanggilnya bisa mengatakannya alih-alih menyamarkannya.
func RilisTerbaru(prarilis bool) (tag string, pra bool, err error) {
	if !prarilis {
		if t, err := tagDari("https://api.github.com/repos/" + UnduhRepo + "/releases/latest"); err == nil && t != "" {
			return t, false, nil
		}
	}
	t, err := tagDari("https://api.github.com/repos/" + UnduhRepo + "/releases?per_page=1")
	if err != nil {
		return "", false, err
	}
	if t == "" {
		return "", false, errors.New("tidak ada rilis di " + UnduhRepo)
	}
	return t, !prarilis, nil
}

// tagDari mengambil tag_name pertama dari sebuah tanggapan GitHub.
//
// Diurai dengan pencarian sederhana, bukan encoding/json: yang dibutuhkan satu
// medan, dan tanggapan rilis GitHub memuat puluhan medan lain yang tidak perlu
// dijelaskan sebagai struct hanya untuk dibuang lagi.
func tagDari(url string) (string, error) {
	c := &http.Client{Timeout: batasTanya}
	resp, err := c.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	isi, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	i := strings.Index(string(isi), `"tag_name"`)
	if i < 0 {
		return "", nil
	}
	sisa := string(isi)[i+len(`"tag_name"`):]
	j := strings.Index(sisa, `"`)
	if j < 0 {
		return "", nil
	}
	sisa = sisa[j+1:]
	k := strings.Index(sisa, `"`)
	if k < 0 {
		return "", nil
	}
	return sisa[:k], nil
}

// unduhRilis mengambil berkas rilis untuk platform host.
//
// Ini jalan keluar bagi mayoritas pengguna, dan selama ini justru merekalah
// yang tidak punya. Yang memasang lewat `curl | sh` tidak punya pohon sumber
// maupun toolchain Go, sehingga memasang dari laptop macOS ke server Linux
// berakhir dengan pesan yang menyuruh menjalankan `make cross` di repo yang
// tidak pernah mereka unduh.
//
// Untuk `up`, versinya diikat ke versi binary LOKAL, bukan ke rilis terbaru:
// perintah itu memasang "anjuran yang sama seperti di mesin ini", dan
// diam-diam memasang versi lain di host membuat perbedaan perilaku antara dua
// mesin menjadi teka-teki yang tidak ada petunjuknya.
func unduhRilis(p Platform, versi string) (Source, func(), error) {
	noop := func() {}

	// Binary dari pohon kerja tidak punya rilis yang bersesuaian. Pemakainya
	// juga hampir pasti sedang berada di dalam pohon sumber, dan di sana
	// pembangunan silang sudah menjadi jalan yang lebih baik.
	if versi == "" || versi == "dev" {
		return Source{}, noop, errTakBisaUnduh
	}

	polos := strings.TrimPrefix(versi, "v")
	// Rilis Windows dikemas zip, sisanya tar.gz — mengikuti kebiasaan tiap
	// sistem, dan itulah yang dihasilkan goreleaser.
	nama := fmt.Sprintf("anjuran_%s_%s_%s.tar.gz", polos, p.OS, p.Arch)
	if p.OS == "windows" {
		nama = fmt.Sprintf("anjuran_%s_%s_%s.zip", polos, p.OS, p.Arch)
	}
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

// Kemajuan menerima laporan kemajuan unduhan. Nil berarti tidak dilaporkan.
//
// Sebuah antarmuka, bukan penulisan langsung ke terminal: paket ini juga
// dipakai `up`, yang laporannya berbentuk lain, dan dipakai uji yang tidak
// boleh mengotori keluarannya.
type Kemajuan func(sudah, total int64)

// pelapor dipasang pemanggil sebelum mengunduh. Disimpan sebagai variabel
// paket karena ia menembus beberapa lapis pemanggilan yang seluruhnya tidak
// punya urusan dengan tampilan.
var pelapor Kemajuan

// LaporkanKemajuan memasang penerima laporan kemajuan unduhan.
func LaporkanKemajuan(f Kemajuan) { pelapor = f }

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

	var r io.Reader = resp.Body
	// Hanya berkas besar yang dilaporkan. checksums.txt beberapa ratus byte,
	// dan batang kemajuan untuknya hanya berkedip sekali lalu hilang.
	if pelapor != nil && resp.ContentLength > 1<<20 {
		r = &pembacaLapor{r: resp.Body, total: resp.ContentLength, lapor: pelapor}
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// pembacaLapor menghitung byte yang lewat lalu melaporkannya.
type pembacaLapor struct {
	r        io.Reader
	total    int64
	sudah    int64
	terakhir time.Time
	lapor    Kemajuan
}

func (p *pembacaLapor) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.sudah += int64(n)

	// Dibatasi sepuluh kali sedetik. Melaporkan setiap potongan berarti
	// menulis ke terminal ribuan kali untuk satu unduhan — cukup untuk
	// membuat pengunduhan itu sendiri terasa lambat.
	if err == io.EOF || time.Since(p.terakhir) > 100*time.Millisecond {
		p.terakhir = time.Now()
		p.lapor(p.sudah, p.total)
	}
	return n, err
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
