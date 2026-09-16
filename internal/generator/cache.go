package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ufhy/anjuran-cli/internal/recall"
)

// Cache menyimpan keluaran generator di disk.
//
// Harus di disk, bukan di memori: anjuran adalah proses baru setiap kali Tab
// ditekan, sehingga cache dalam memori tidak akan pernah terpakai sekali pun.
// Inilah yang membuat Tab berulang pada `kubectl get pods` terasa instan
// alih-alih memanggil cluster lagi.
type Cache struct {
	Dir string
	// Now bisa diganti saat pengujian.
	Now func() time.Time
}

// NewCache membuat cache di direktori cache milik pengguna. Mengembalikan nil
// bila lokasinya tidak tersedia; pemanggil memperlakukan nil sebagai
// "jalan tanpa cache", bukan sebagai kegagalan.
func NewCache() *Cache {
	base := recall.CacheDir()
	if base == "" {
		return nil
	}
	dir := filepath.Join(base, "gen")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil
	}
	return &Cache{Dir: dir, Now: time.Now}
}

func (c *Cache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// key membedakan entri berdasarkan argv DAN direktori kerja. Direktori ikut
// dihitung karena `git branch` menjawab berbeda di setiap repo.
func (c *Cache) key(argv []string, dir string) string {
	h := sha256.New()
	h.Write([]byte(dir))
	for _, a := range argv {
		h.Write([]byte{0})
		h.Write([]byte(a))
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// Get mengambil entri yang belum kedaluwarsa.
func (c *Cache) Get(argv []string, dir string) (string, bool) {
	if c == nil {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(c.Dir, c.key(argv, dir)))
	if err != nil {
		return "", false
	}

	// Baris pertama adalah waktu kedaluwarsa dalam detik Unix.
	nl := strings.IndexByte(string(b), '\n')
	if nl < 0 {
		return "", false
	}
	exp, err := strconv.ParseInt(string(b[:nl]), 10, 64)
	if err != nil || c.now().Unix() >= exp {
		return "", false
	}
	return string(b[nl+1:]), true
}

// Put menyimpan keluaran dengan masa berlaku ttl.
func (c *Cache) Put(argv []string, dir, out string, ttl time.Duration) {
	if c == nil || ttl <= 0 {
		return
	}
	path := filepath.Join(c.Dir, c.key(argv, dir))
	body := strconv.FormatInt(c.now().Add(ttl).Unix(), 10) + "\n" + out

	// Ditulis lewat berkas sementara lalu dipindahkan, supaya proses anjuran lain
	// yang membaca bersamaan tidak pernah melihat isi yang setengah jadi.
	tmp, err := os.CreateTemp(c.Dir, "tmp-*")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	os.Rename(tmp.Name(), path)
}

// Sweep membuang entri yang sudah kedaluwarsa.
//
// Dipanggil sesekali, bukan setiap kali: menelusuri direktori pada setiap
// penekanan Tab justru menambah kerja pada jalur yang paling ingin kita jaga.
func (c *Cache) Sweep() {
	if c == nil {
		return
	}
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return
	}
	now := c.now()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		// Masa berlaku terpanjang yang kita izinkan jauh di bawah sehari,
		// jadi berkas yang lebih tua dari itu pasti sudah tidak berguna.
		if now.Sub(info.ModTime()) > 24*time.Hour {
			os.Remove(filepath.Join(c.Dir, e.Name()))
		}
	}
}
