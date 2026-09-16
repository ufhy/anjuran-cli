package recall

import (
	"os"
	"path/filepath"
)

// EnvCacheDir menimpa lokasi cache anjuran.
//
// Ada supaya pengujian bisa berjalan tanpa mengotori cache pengguna, dan
// supaya pemasangan di lingkungan yang direktori rumahnya hanya-baca tetap
// bisa mengarahkannya ke tempat lain.
const EnvCacheDir = "ANJURAN_CACHE_DIR"

// CacheDir mengembalikan direktori cache anjuran, membuatnya bila perlu.
// Mengembalikan string kosong bila tidak ada tempat yang bisa dipakai.
func CacheDir() string {
	dir := os.Getenv(EnvCacheDir)
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(base, "anjuran")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	return dir
}
