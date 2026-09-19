// Package spec memodelkan subset skema completion-spec milik Fig
// (github.com/withfig/autocomplete, MIT). Struktur sengaja dibuat
// kompatibel supaya transpiler TS->JSON di tahap berikutnya tidak
// perlu mengubah bentuk data.
package spec

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// Separator menampung field yang di skema Fig boleh berupa bool maupun string.
//
// Bentuk bool berarti "wajib menempel, pakai tanda sama dengan"; bentuk string
// menyebutkan karakter pemisahnya sendiri, misalnya ":" pada beberapa perintah.
// Memperlakukannya sebagai bool saja membuat SELURUH berkas spec gagal diurai —
// bukan sekadar satu opsi yang hilang, melainkan perintah itu mati sama sekali.
type Separator struct {
	Required bool
	// Char adalah karakter pemisahnya; kosong berarti tanda sama dengan.
	Char string
}

func (s *Separator) UnmarshalJSON(b []byte) error {
	var flag bool
	if err := json.Unmarshal(b, &flag); err == nil {
		s.Required = flag
		return nil
	}
	var text string
	if err := json.Unmarshal(b, &text); err == nil {
		s.Required = text != ""
		s.Char = text
		return nil
	}
	// Bentuk lain diabaikan diam-diam. Satu field yang tidak dikenali tidak
	// boleh menjatuhkan seluruh spec.
	return nil
}

func (s Separator) MarshalJSON() ([]byte, error) {
	if s.Char != "" {
		return json.Marshal(s.Char)
	}
	return json.Marshal(s.Required)
}

// Rune mengembalikan karakter pemisah yang dipakai.
func (s Separator) Rune() string {
	if s.Char != "" {
		return s.Char
	}
	return "="
}

// Suggestion adalah satu entri statis yang bisa ditawarkan untuk sebuah argumen.
type Suggestion struct {
	Name        Names  `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	// InsertValue menggantikan teks yang disisipkan bila berbeda dari nama.
	// Boleh memuat penanda {cursor} untuk menentukan posisi kursor akhir.
	InsertValue string `json:"insertValue,omitempty"`
	// Priority mengikuti konvensi Fig: default 50, makin besar makin diutamakan.
	Priority int `json:"priority,omitempty"`
	// Hidden menandai entri yang hanya cocok saat diketik persis.
	Hidden     bool `json:"hidden,omitempty"`
	Deprecated bool `json:"deprecated,omitempty"`
	// WhenFile membuat entri ini hanya ditawarkan bila berkas bernama itu ada
	// di direktori kerja.
	WhenFile string `json:"whenFile,omitempty"`
}

// Generator mendeskripsikan sumber kandidat dinamis secara DEKLARATIF.
// Di Fig field ini berupa closure JavaScript; kita hanya menerima bentuk
// yang bisa dieksekusi tanpa mesin JS. Tahap 1 belum menjalankannya —
// engine hanya membawa metadatanya.
type Generator struct {
	// Trusted menandai generator yang berasal dari spec buatan tangan —
	// tambalan bawaan anjuran atau milik pengguna sendiri — bukan dari 1.472 berkas
	// hasil transpile korpus pihak ketiga.
	//
	// Sengaja TIDAK bisa diisi dari JSON: kalau bisa, berkas spec mana pun
	// tinggal menyatakan dirinya tepercaya dan seluruh kebijakannya runtuh.
	// Nilainya dipasang oleh registry berdasarkan direktori asal berkasnya.
	Trusted bool `json:"-"`

	// Script adalah perintah yang dijalankan untuk menghasilkan kandidat,
	// sudah dalam bentuk argv (tanpa shell) supaya tidak bisa di-inject.
	Script []string `json:"script,omitempty"`
	// SplitOn memecah stdout menjadi baris kandidat, default "\n".
	SplitOn string `json:"splitOn,omitempty"`
	// Template merujuk sumber bawaan seperti "filepaths" atau "folders".
	Template []string `json:"template,omitempty"`
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
	Default     string       `json:"default,omitempty"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
	Generators  []Generator  `json:"generators,omitempty"`
	// Template merujuk sumber bawaan seperti "filepaths" atau "folders".
	Template []string `json:"template,omitempty"`
}

// Option adalah flag seperti -v atau --verbose.
type Option struct {
	Name        Names  `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	InsertValue string `json:"insertValue,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Args        []Arg  `json:"args,omitempty"`
	// IsPersistent membuat opsi ini juga berlaku di seluruh subcommand turunan.
	IsPersistent bool `json:"isPersistent,omitempty"`
	IsRepeatable bool `json:"isRepeatable,omitempty"`
	// RequiresSeparator menandai opsi yang nilainya WAJIB ditulis menempel,
	// misalnya --jobs=4 dan bukan --jobs 4. Tanpa ini, penelusuran akan salah
	// menelan token berikutnya sebagai argumen opsi.
	RequiresSeparator Separator `json:"requiresSeparator,omitempty"`
	// ExclusiveOn menyembunyikan opsi ini bila salah satu nama di sini sudah dipakai.
	ExclusiveOn []string `json:"exclusiveOn,omitempty"`
	// DependsOn menyembunyikan opsi ini sampai opsi yang disebut sudah dipakai.
	DependsOn   []string `json:"dependsOn,omitempty"`
	IsDangerous bool     `json:"isDangerous,omitempty"`
	Hidden      bool     `json:"hidden,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
}

// Subcommand adalah simpul rekursif: sebuah spec akar juga berbentuk ini.
type Subcommand struct {
	Name        Names        `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Description string       `json:"description,omitempty"`
	InsertValue string       `json:"insertValue,omitempty"`
	Priority    int          `json:"priority,omitempty"`
	Subcommands []Subcommand `json:"subcommands,omitempty"`
	Options     []Option     `json:"options,omitempty"`
	Args        []Arg        `json:"args,omitempty"`
	// LoadSpec merujuk berkas spec lain yang memuat isi subcommand ini,
	// misalnya "aws/s3". Isinya baru dibaca saat penelusuran benar-benar
	// sampai ke simpul ini; tanpa itu, satu spec aws berarti membaca puluhan
	// megabyte untuk melengkapi satu kata.
	LoadSpec string `json:"loadSpec,omitempty"`
	// WhenFile membuat entri ini hanya ditawarkan bila berkas atau direktori
	// bernama itu ada di direktori kerja.
	//
	// Banyak perintah punya subcommand yang hanya bermakna di dalam proyek
	// tertentu: "php artisan" hanya ada di proyek Laravel, "npm run" hanya
	// berguna bila ada package.json. Menawarkannya di mana-mana membuat
	// daftarnya berbohong tentang apa yang sebenarnya bisa dijalankan.
	WhenFile string `json:"whenFile,omitempty"`
	// Contoh adalah baris perintah yang HARUS menjawab sesuatu.
	//
	// Hanya dipakai spec tulisan tangan di extra/, dan ada demi ujinya: spec
	// yang diterima dari orang lain harus bisa dibuktikan menjawab, bukan
	// sekadar bisa diurai. Menaruhnya di dalam berkas specnya sendiri membuat
	// sumbangan cukup satu berkas — contoh yang terpisah akan lupa diperbarui
	// saat specnya berubah.
	//
	// Namanya berawalan anjuran supaya tidak pernah bentrok dengan bidang Fig.
	Contoh []string `json:"anjuranContoh,omitempty"`

	RequiresSubcommand bool `json:"requiresSubcommand,omitempty"`
	IsDangerous        bool `json:"isDangerous,omitempty"`
	Hidden             bool `json:"hidden,omitempty"`
	Deprecated         bool `json:"deprecated,omitempty"`
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

// Registry memuat spec dari sebuah direktori, dengan cache per proses.
//
// Berkas boleh berupa <nama>.json maupun <nama>.json.gz. Bentuk terkompresi
// dipakai untuk distribusi: seluruh spec Fig berukuran 45 MB sebagai JSON
// polos tetapi hanya sekitar 5 MB ter-gzip, dan itu berarti jauh lebih ringan
// saat ikut dikirim ke host remote lewat SSH.
// Registry mencari di beberapa direktori secara berurutan. Direktori pertama
// yang memuat berkasnya menang.
//
// Jalur ganda ini bukan kemewahan: direktori specs/ bawaan DIHASILKAN dari
// paket Fig dan ditimpa setiap kali dibangun ulang, jadi spec buatan sendiri —
// untuk CLI internal yang tidak akan pernah ada di repo Fig — membutuhkan
// tempat yang tidak ikut terhapus, sekaligus cara menimpa spec bawaan yang
// dianggap kurang tepat.
type Registry struct {
	dirs  []string
	cache map[string]*Subcommand
	// trusted menandai direktori yang isinya ditulis tangan dan ditinjau.
	trusted map[string]bool
}

// Trust menandai sebuah direktori sebagai tepercaya.
//
// Yang tepercaya bukan binernya, melainkan asal spec-nya. Tambalan bawaan dan
// spec milik pengguna ditulis dan ditinjau satu per satu; spec hasil transpile
// berasal dari korpus 1.472 berkas pihak ketiga yang tidak pernah dibaca
// seorang pun. Perbedaan itulah yang menentukan apakah sebuah generator boleh
// memanggil interpreter.
func (r *Registry) Trust(dirs ...string) {
	if r.trusted == nil {
		r.trusted = map[string]bool{}
	}
	for _, d := range dirs {
		if d != "" {
			r.trusted[d] = true
		}
	}
}

// NewRegistry membuat registry dengan satu direktori.
func NewRegistry(dir string) *Registry {
	return NewRegistryDirs(dir)
}

// NewRegistryDirs membuat registry dengan urutan pencarian.
func NewRegistryDirs(dirs ...string) *Registry {
	clean := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d != "" {
			clean = append(clean, d)
		}
	}
	return &Registry{dirs: clean, cache: map[string]*Subcommand{}}
}

// Dirs mengembalikan urutan direktori yang dicari.
func (r *Registry) Dirs() []string { return r.dirs }

// Load mengembalikan spec untuk sebuah nama perintah. Perintah yang tidak
// punya spec mengembalikan (nil, nil) — bukan error, karena mayoritas
// perintah di PATH memang tidak akan pernah punya spec.
func (r *Registry) Load(command string) (*Subcommand, error) {
	if command == "" || strings.ContainsAny(command, `/\`) {
		return nil, nil
	}
	return r.loadPath(command)
}

// Resolve mengembalikan isi sebenarnya sebuah subcommand. Untuk simpul biasa
// nilainya adalah sc itu sendiri; untuk simpul ber-loadSpec, berkas rujukannya
// dibaca lebih dulu.
//
// Pemuatan ditunda sampai titik ini secara sengaja. Spec aws terdiri atas
// ratusan berkas; membacanya sekaligus hanya untuk melengkapi satu kata akan
// menghabiskan anggaran waktu yang kita jaga ketat.
func (r *Registry) Resolve(sc *Subcommand) (*Subcommand, error) {
	if sc == nil || sc.LoadSpec == "" {
		return sc, nil
	}

	loaded, err := r.loadPath(sc.LoadSpec)
	if err != nil || loaded == nil {
		// Rujukan yang tidak ada tidak boleh mematikan completion; simpul
		// dipakai apa adanya.
		return sc, nil
	}

	// Nama dan deskripsi milik simpul pemanggil tetap dipertahankan, karena
	// itulah yang dikenal pengguna di baris perintah.
	merged := *loaded
	merged.Name = sc.Name
	if sc.Description != "" {
		merged.Description = sc.Description
	}
	merged.LoadSpec = ""
	return &merged, nil
}

// loadPath membaca satu berkas spec berdasarkan path relatif tanpa ekstensi.
// loadPath membaca spec dari SELURUH direktori pencarian lalu menggabungkannya.
//
// Direktori yang lebih diutamakan menambal yang di bawahnya, bukan
// menggantikannya: spec bawaan lengkap pada bagian opsi, dan tambalan biasanya
// hanya mengisi argumen yang kosong. Mengambil yang pertama saja akan
// menghilangkan ribuan opsi hanya karena satu argumen ditambal.
func (r *Registry) loadPath(rel string) (*Subcommand, error) {
	if sc, ok := r.cache[rel]; ok {
		return sc, nil
	}

	if merged, err := r.loadMerged(rel); err != nil || merged != nil {
		if merged != nil {
			r.cache[rel] = merged
		}
		return merged, err
	}

	b, err := r.readSpecFile(rel)
	if err != nil {
		return nil, err
	}
	if b == nil {
		// Sebagian perintah hanya disimpan Fig sebagai spec berversi, misalnya
		// az yang ada sebagai az/2.53.0 tanpa az di akar sama sekali.
		if v := r.resolveVersioned(rel); v != "" {
			b, err = r.readSpecFile(v)
			if err != nil {
				return nil, err
			}
		}
	}
	if b == nil {
		r.cache[rel] = nil
		return nil, nil
	}

	var sc Subcommand
	if err := json.Unmarshal(b, &sc); err != nil {
		return nil, fmt.Errorf("spec %s: %w", rel, err)
	}
	r.cache[rel] = &sc
	return &sc, nil
}

// semverName mencocokkan nama berkas spec berversi seperti "2.53.0".
var semverName = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?$`)

// resolveVersioned mencari spec berversi tertinggi untuk sebuah perintah.
//
// Fig menentukan versi yang tepat dengan menjalankan perintahnya lebih dulu.
// Di sini versi tertinggi yang dipilih: tanpa mengeksekusi apa pun, itu
// tebakan terbaik yang tersedia, dan selisih antar versi minor pada spec
// hampir selalu berupa penambahan subcommand.
func (r *Registry) resolveVersioned(command string) string {
	if strings.Contains(command, "/") {
		return ""
	}

	best := ""
	var bestNum [3]int
	for _, dir := range r.dirs {
		entries, err := os.ReadDir(filepath.Join(dir, command))
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".gz"), ".json")
			m := semverName.FindStringSubmatch(name)
			if m == nil {
				continue
			}
			var num [3]int
			for i := 0; i < 3; i++ {
				n, _ := strconv.Atoi(m[i+1])
				num[i] = n
			}
			if best == "" || num[0] > bestNum[0] ||
				(num[0] == bestNum[0] && num[1] > bestNum[1]) ||
				(num[0] == bestNum[0] && num[1] == bestNum[1] && num[2] > bestNum[2]) {
				best, bestNum = command+"/"+name, num
			}
		}
		if best != "" {
			// Direktori yang lebih diutamakan sudah menjawab; jangan dicampur
			// dengan versi dari direktori berikutnya.
			return best
		}
	}
	return best
}

// loadMerged membaca spec bernama sama dari setiap direktori pencarian lalu
// menggabungkannya, dimulai dari yang paling rendah prioritasnya supaya
// direktori paling depan menjadi lapisan terakhir yang menimpa.
func (r *Registry) loadMerged(rel string) (*Subcommand, error) {
	type layer struct {
		body    []byte
		trusted bool
	}
	var found []layer
	for _, dir := range r.dirs {
		b, err := r.readSpecIn(dir, rel)
		if err != nil {
			return nil, err
		}
		if b != nil {
			found = append(found, layer{b, r.trusted[dir]})
		}
	}
	if len(found) == 0 {
		return nil, nil
	}

	var merged *Subcommand
	for i := len(found) - 1; i >= 0; i-- {
		var sc Subcommand
		if err := json.Unmarshal(found[i].body, &sc); err != nil {
			return nil, fmt.Errorf("spec %s: %w", rel, err)
		}
		if found[i].trusted {
			markTrusted(&sc)
		}
		merged = merge(merged, &sc)
	}
	return merged, nil
}

// readSpecIn membaca satu spec dari sebuah direktori, mendahulukan bentuk
// terkompresi. Mengembalikan (nil, nil) bila tidak ada di direktori itu.
func (r *Registry) readSpecIn(dir, rel string) ([]byte, error) {
	clean := filepath.Clean("/" + filepath.FromSlash(rel))[1:]
	if clean == "" || strings.HasPrefix(clean, "..") {
		return nil, nil
	}
	base := filepath.Join(dir, clean)

	if b, err := os.ReadFile(base + ".json.gz"); err == nil {
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("spec %s: %w", rel, err)
		}
		defer zr.Close()
		return io.ReadAll(zr)
	}

	b, err := os.ReadFile(base + ".json")
	if err == nil {
		return b, nil
	}
	if takAdaSpec(err) {
		return nil, nil
	}
	return nil, err
}

// takAdaSpec menjawab apakah galat ini berarti "spec-nya memang tidak ada".
//
// Bukan hanya IsNotExist. Nama spec disusun dari nama perintah yang sedang
// diketik, dan orang mengetik apa saja — termasuk karakter yang tidak sah
// sebagai nama berkas. Windows menolaknya dengan ENOTDIR atau EINVAL alih-alih
// ENOENT, dan memperlakukan itu sebagai kegagalan membuat SELURUH completion
// mati hanya karena satu karakter aneh di baris perintah.
//
// Ditemukan oleh penyapuan acak: `for <tab><tab>`日*|🚀日~="<<=&` menghasilkan
// nama berkas yang sah di Unix tetapi tidak di Windows.
func takAdaSpec(err error) bool {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) {
		return true
	}
	// Sisanya khas per sistem dan tidak punya nilai errors.Is yang portabel —
	// ENOTDIR di Unix, ERROR_INVALID_NAME di Windows. Yang menentukan: galat
	// ini muncul saat MEMBUKA nama yang kita susun sendiri, jadi apa pun
	// sebabnya, artinya tetap "tidak ada spec di sana".
	var e *os.PathError
	return errors.As(err, &e)
}

// readSpecFile menelusuri direktori sesuai urutan, mencari bentuk terkompresi
// lebih dulu lalu JSON polos. Mengembalikan (nil, nil) bila spec memang tidak
// ada di mana pun.
func (r *Registry) readSpecFile(rel string) ([]byte, error) {
	// Tolak path yang mencoba keluar dari direktori spec. Berkas spec berasal
	// dari pihak ketiga, jadi rujukan loadSpec diperlakukan sebagai masukan
	// yang tidak dipercaya.
	clean := filepath.Clean("/" + filepath.FromSlash(rel))[1:]
	if clean == "" || strings.HasPrefix(clean, "..") {
		return nil, nil
	}

	for _, dir := range r.dirs {
		base := filepath.Join(dir, clean)

		if b, err := os.ReadFile(base + ".json.gz"); err == nil {
			zr, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				return nil, fmt.Errorf("spec %s: %w", rel, err)
			}
			defer zr.Close()
			return io.ReadAll(zr)
		}

		b, err := os.ReadFile(base + ".json")
		if err == nil {
			return b, nil
		}
		if !takAdaSpec(err) {
			return nil, err
		}
	}
	return nil, nil
}
