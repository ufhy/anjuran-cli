// Package recall mengingat kandidat yang pernah dipilih pengguna.
//
// Ini yang membuat completion IDE terasa mengenal kebiasaan: VS Code
// menyebutnya suggestSelection, dengan pilihan "recentlyUsed" dan
// "recentlyUsedByPrefix". Yang pernah kamu pilih untuk sebuah awalan akan
// tersorot lebih dulu di kali berikutnya, alih-alih kamu menekan panah ke
// entri yang sama setiap hari.
//
// Ingatannya sengaja SEMPIT: kunci mencakup perintah dan awalan yang diketik,
// bukan sekadar nama kandidat. "git c" yang biasanya berakhir di commit tidak
// boleh mengubah urutan "docker c".
package recall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// maxEntri membatasi jumlah ingatan yang disimpan.
//
// Berkasnya dibaca pada setiap penekanan tombol pemicu, jadi ia harus tetap
// kecil. Yang paling lama tidak dipakai dibuang lebih dulu.
const maxEntri = 2000

// Store adalah ingatan yang tersimpan di disk.
type Store struct {
	path  string
	data  map[string]entry
	dirty bool
	// Now bisa diganti saat pengujian.
	Now func() time.Time
}

type entry struct {
	Name string `json:"n"`
	At   int64  `json:"t"`
}

// Open membaca ingatan dari direktori cache pengguna.
//
// Mengembalikan store yang tetap bisa dipakai meski berkasnya gagal dibaca:
// ingatan yang hilang hanya berarti urutan kembali ke bawaan, bukan kegagalan
// yang perlu diberitahukan.
func Open() *Store {
	s := &Store{data: map[string]entry{}, Now: time.Now}

	dir := CacheDir()
	if dir == "" {
		return s
	}
	s.path = filepath.Join(dir, "picks.json")

	b, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	json.Unmarshal(b, &s.data)
	return s
}

// Preferred mengembalikan kandidat yang terakhir dipilih untuk sebuah kunci.
func (s *Store) Preferred(key string) string {
	if s == nil || key == "" {
		return ""
	}
	return s.data[key].Name
}

// Record mencatat pilihan pengguna.
func (s *Store) Record(key, name string) {
	if s == nil || key == "" || name == "" {
		return
	}
	if e, ok := s.data[key]; ok && e.Name == name {
		// Nama yang sama tetap menyegarkan waktunya, supaya tidak ikut
		// terbuang saat ingatan dipangkas.
		s.data[key] = entry{Name: name, At: s.now()}
		s.dirty = true
		return
	}
	s.data[key] = entry{Name: name, At: s.now()}
	s.dirty = true
}

func (s *Store) now() int64 {
	if s.Now != nil {
		return s.Now().Unix()
	}
	return time.Now().Unix()
}

// Save menulis ingatan ke disk bila ada yang berubah.
//
// Ditulis lewat berkas sementara lalu dipindahkan, supaya proses anjuran lain yang
// membaca bersamaan tidak pernah melihat isi yang setengah jadi.
func (s *Store) Save() {
	if s == nil || !s.dirty || s.path == "" {
		return
	}
	s.prune()

	b, err := json.Marshal(s.data)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "picks-*")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	os.Rename(tmp.Name(), s.path)
}

// prune membuang ingatan yang paling lama tidak dipakai.
func (s *Store) prune() {
	if len(s.data) <= maxEntri {
		return
	}
	type kv struct {
		key string
		at  int64
	}
	all := make([]kv, 0, len(s.data))
	for k, e := range s.data {
		all = append(all, kv{k, e.At})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at > all[j].at })
	for _, x := range all[maxEntri:] {
		delete(s.data, x.key)
	}
}

// Key merangkai kunci ingatan dari konteks pengetikan.
//
// Perintah DAN awalan ikut masuk, karena kebiasaan itu melekat pada keduanya:
// "git c" yang biasanya berakhir di commit tidak boleh mengubah urutan
// "docker c".
func Key(command, prefix string) string {
	if command == "" {
		return ""
	}
	return command + "\x00" + prefix
}
