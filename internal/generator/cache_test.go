package generator

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func cacheUji(t *testing.T) (*Cache, *time.Time) {
	t.Helper()
	now := time.Now()
	c := &Cache{Dir: t.TempDir(), Now: func() time.Time { return now }}
	return c, &now
}

func TestCacheSimpanDanAmbil(t *testing.T) {
	c, _ := cacheUji(t)
	argv := []string{"git", "branch"}

	if _, ok := c.Get(argv, "/repo"); ok {
		t.Fatal("cache kosong seharusnya meleset")
	}

	c.Put(argv, "/repo", "a\nb\n", time.Minute)
	out, ok := c.Get(argv, "/repo")
	if !ok || out != "a\nb\n" {
		t.Errorf("Get = %q, %v; mau isi tersimpan", out, ok)
	}
}

// Direktori kerja ikut menjadi kunci, karena `git branch` menjawab berbeda
// di setiap repo. Tanpa ini, pindah repo akan menampilkan branch repo lama.
func TestCacheDibedakanPerDirektori(t *testing.T) {
	c, _ := cacheUji(t)
	argv := []string{"git", "branch"}

	c.Put(argv, "/repo-a", "a\n", time.Minute)
	if _, ok := c.Get(argv, "/repo-b"); ok {
		t.Error("entri repo lain seharusnya tidak terpakai")
	}
}

func TestCacheDibedakanPerArgv(t *testing.T) {
	c, _ := cacheUji(t)
	c.Put([]string{"git", "branch"}, "/r", "a\n", time.Minute)
	if _, ok := c.Get([]string{"git", "tag"}, "/r"); ok {
		t.Error("argv berbeda seharusnya kunci berbeda")
	}
}

func TestCacheKedaluwarsa(t *testing.T) {
	c, now := cacheUji(t)
	argv := []string{"git", "branch"}

	c.Put(argv, "/r", "a\n", 10*time.Second)
	if _, ok := c.Get(argv, "/r"); !ok {
		t.Fatal("entri baru seharusnya kena")
	}

	*now = now.Add(11 * time.Second)
	if _, ok := c.Get(argv, "/r"); ok {
		t.Error("entri kedaluwarsa seharusnya meleset")
	}
}

func TestCacheTTLNolTidakMenyimpan(t *testing.T) {
	c, _ := cacheUji(t)
	c.Put([]string{"x"}, "/r", "a\n", 0)
	if _, ok := c.Get([]string{"x"}, "/r"); ok {
		t.Error("ttl nol seharusnya tidak menyimpan apa pun")
	}
}

// Proses uf lain bisa membaca bersamaan, jadi berkas tidak boleh pernah
// terlihat setengah jadi.
func TestCacheTidakMeninggalkanBerkasSementara(t *testing.T) {
	c, _ := cacheUji(t)
	c.Put([]string{"git", "branch"}, "/r", "a\n", time.Minute)

	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) > 4 && e.Name()[:4] == "tmp-" {
			t.Errorf("berkas sementara tertinggal: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("mau tepat satu berkas, dapat %d", len(entries))
	}
}

func TestCacheNilAman(t *testing.T) {
	var c *Cache
	if _, ok := c.Get([]string{"x"}, ""); ok {
		t.Error("cache nil seharusnya selalu meleset")
	}
	c.Put([]string{"x"}, "", "a", time.Minute) // tidak boleh panik
	c.Sweep()
}

func TestCacheIsiRusakDiabaikan(t *testing.T) {
	c, _ := cacheUji(t)
	argv := []string{"git", "branch"}
	os.WriteFile(filepath.Join(c.Dir, c.key(argv, "/r")), []byte("bukan-angka\nisi"), 0o600)
	if _, ok := c.Get(argv, "/r"); ok {
		t.Error("berkas cache rusak seharusnya diabaikan, bukan dipakai")
	}
}

func TestSweepMembuangBerkasLama(t *testing.T) {
	c, now := cacheUji(t)
	c.Put([]string{"x"}, "/r", "a\n", time.Minute)

	lama := now.Add(-48 * time.Hour)
	entries, _ := os.ReadDir(c.Dir)
	for _, e := range entries {
		os.Chtimes(filepath.Join(c.Dir, e.Name()), lama, lama)
	}

	c.Sweep()
	if entries, _ := os.ReadDir(c.Dir); len(entries) != 0 {
		t.Errorf("berkas lama seharusnya dibuang, tersisa %d", len(entries))
	}
}
