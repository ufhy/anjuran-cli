// Package spec memodelkan subset skema completion-spec milik Fig
// (github.com/withfig/autocomplete, MIT). Struktur sengaja dibuat
// kompatibel supaya transpiler TS->JSON di tahap berikutnya tidak
// perlu mengubah bentuk data.
package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Names menampung field yang di skema Fig boleh berupa string tunggal
// ataupun array string, misalnya "name": "-v" dan "name": ["-v","--verbose"].
type Names []string

func (n *Names) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*n = Names{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("name harus string atau []string: %w", err)
	}
	*n = Names(many)
	return nil
}

// Primary mengembalikan nama kanonik (yang ditampilkan di dropdown).
func (n Names) Primary() string {
	if len(n) == 0 {
		return ""
	}
	return n[0]
}

func (n Names) Has(s string) bool {
	for _, v := range n {
		if v == s {
			return true
		}
	}
	return false
}

// Suggestion adalah satu entri statis yang bisa ditawarkan untuk sebuah argumen.
type Suggestion struct {
	Name        Names  `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	// Hidden menandai entri yang hanya cocok saat diketik persis.
	Hidden bool `json:"hidden,omitempty"`
}

// Generator mendeskripsikan sumber kandidat dinamis secara DEKLARATIF.
// Di Fig field ini berupa closure JavaScript; kita hanya menerima bentuk
// yang bisa dieksekusi tanpa mesin JS. Tahap 1 belum menjalankannya —
// engine hanya membawa metadatanya.
type Generator struct {
	// Script adalah perintah yang dijalankan untuk menghasilkan kandidat,
	// sudah dalam bentuk argv (tanpa shell) supaya tidak bisa di-inject.
	Script []string `json:"script,omitempty"`
	// SplitOn memecah stdout menjadi baris kandidat, default "\n".
	SplitOn string `json:"splitOn,omitempty"`
	// Trim membuang spasi di tiap kandidat.
	Trim bool `json:"trim,omitempty"`
	// CacheTTL dalam detik; 0 berarti tidak di-cache.
	CacheTTL int `json:"cacheTtl,omitempty"`
}

// Arg mendeskripsikan satu argumen posisional.
type Arg struct {
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	IsOptional  bool         `json:"isOptional,omitempty"`
	IsVariadic  bool         `json:"isVariadic,omitempty"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
	Generators  []Generator  `json:"generators,omitempty"`
	// Template merujuk sumber bawaan seperti "filepaths" atau "folders".
	Template []string `json:"template,omitempty"`
}

// Option adalah flag seperti -v atau --verbose.
type Option struct {
	Name        Names  `json:"name"`
	Description string `json:"description,omitempty"`
	Args        []Arg  `json:"args,omitempty"`
	// IsPersistent membuat opsi ini juga berlaku di seluruh subcommand turunan.
	IsPersistent bool `json:"isPersistent,omitempty"`
	IsRepeatable bool `json:"isRepeatable,omitempty"`
	// ExclusiveOn menyembunyikan opsi ini bila salah satu nama di sini sudah dipakai.
	ExclusiveOn []string `json:"exclusiveOn,omitempty"`
	Hidden      bool     `json:"hidden,omitempty"`
}

// Subcommand adalah simpul rekursif: sebuah spec akar juga berbentuk ini.
type Subcommand struct {
	Name        Names        `json:"name"`
	Description string       `json:"description,omitempty"`
	Subcommands []Subcommand `json:"subcommands,omitempty"`
	Options     []Option     `json:"options,omitempty"`
	Args        []Arg        `json:"args,omitempty"`
	Hidden      bool         `json:"hidden,omitempty"`
}

// FindSubcommand mencari subcommand berdasarkan nama atau alias.
func (s *Subcommand) FindSubcommand(name string) *Subcommand {
	for i := range s.Subcommands {
		if s.Subcommands[i].Name.Has(name) {
			return &s.Subcommands[i]
		}
	}
	return nil
}

// FindOption mencari opsi berdasarkan nama persis, termasuk bentuk --opt=value.
func (s *Subcommand) FindOption(name string) *Option {
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	for i := range s.Options {
		if s.Options[i].Name.Has(name) {
			return &s.Options[i]
		}
	}
	return nil
}

// Registry memuat spec dari direktori berisi <command>.json.
type Registry struct {
	dir   string
	cache map[string]*Subcommand
}

func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir, cache: map[string]*Subcommand{}}
}

// Load mengembalikan spec untuk sebuah nama perintah. Perintah yang tidak
// punya spec mengembalikan (nil, nil) — bukan error, karena mayoritas
// perintah di PATH memang tidak akan pernah punya spec.
func (r *Registry) Load(command string) (*Subcommand, error) {
	if command == "" || strings.ContainsAny(command, `/\`) {
		return nil, nil
	}
	if sc, ok := r.cache[command]; ok {
		return sc, nil
	}
	path := filepath.Join(r.dir, command+".json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		r.cache[command] = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sc Subcommand
	if err := json.Unmarshal(b, &sc); err != nil {
		return nil, fmt.Errorf("spec %s: %w", path, err)
	}
	r.cache[command] = &sc
	return &sc, nil
}
