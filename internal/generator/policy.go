// Package generator menjalankan sumber kandidat dinamis milik sebuah spec.
//
// Seluruh paket ini berangkat dari satu kenyataan: menekan Tab akan
// MENJALANKAN PERINTAH. Bukan membaca berkas, bukan memanggil pustaka —
// menjalankan proses, di direktori kerja pengguna, dengan hak aksesnya.
// Karena itu keputusan boleh-tidaknya dipisah ke berkas ini, terpisah dari
// mekanisme eksekusinya, supaya bisa dibaca dan diuji sebagai satu kebijakan
// yang utuh.
package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Decision adalah hasil penilaian sebuah generator.
type Decision struct {
	Allowed bool
	// Reason menjelaskan penolakan dengan kalimat yang bisa ditampilkan.
	Reason string
}

func allow() Decision                  { return Decision{Allowed: true} }
func deny(f string, a ...any) Decision { return Decision{Reason: fmt.Sprintf(f, a...)} }

// interpreters adalah program yang menerima kode sebagai argumen.
//
// Daftar ini ada karena bentuk argv TIDAK dengan sendirinya mencegah eksekusi
// shell: 194 generator di paket spec Fig berisi ["bash","-c","<skrip>"], yang
// berarti shell tetap masuk hanya lewat pintu lain. Menyaring berdasarkan
// argv[0] saja tidak cukup; interpreter harus ditolak secara eksplisit.
var interpreters = map[string]bool{
	"ash": true, "bash": true, "csh": true, "dash": true, "fish": true,
	"ksh": true, "pwsh": true, "sh": true, "tcsh": true, "zsh": true,
	"powershell": true, "cmd": true, "command": true,

	"awk": true, "gawk": true, "perl": true, "php": true, "python": true,
	"python2": true, "python3": true, "ruby": true, "node": true, "deno": true,
	"bun": true, "osascript": true, "env": true, "nohup": true, "xargs": true,
	"eval": true, "exec": true, "sudo": true, "doas": true, "su": true,
}

// Policy memutuskan generator mana yang boleh dijalankan.
type Policy struct {
	// Command adalah perintah yang sedang dilengkapi, misalnya "git".
	Command string
	// Allow adalah biner tambahan yang diizinkan selain Command.
	Allow map[string]bool
	// Enabled mematikan seluruh generator bila bernilai false.
	Enabled bool
	// IsRoot menandai proses berjalan sebagai root.
	IsRoot bool
	// AllowRoot mengizinkan generator tetap berjalan sebagai root.
	AllowRoot bool
}

// Nama variabel lingkungan yang mengatur kebijakan ini.
const (
	EnvDisable   = "UF_NO_GENERATORS"
	EnvAllow     = "UF_GENERATOR_ALLOW"
	EnvAllowRoot = "UF_GENERATOR_ALLOW_ROOT"
	EnvTimeout   = "UF_GENERATOR_TIMEOUT"
)

// PolicyFromEnv menyusun kebijakan dari lingkungan proses.
func PolicyFromEnv(command string) Policy {
	p := Policy{
		Command:   command,
		Allow:     map[string]bool{},
		Enabled:   os.Getenv(EnvDisable) == "",
		IsRoot:    os.Geteuid() == 0,
		AllowRoot: os.Getenv(EnvAllowRoot) != "",
	}
	for _, name := range strings.Split(os.Getenv(EnvAllow), ",") {
		if name = strings.TrimSpace(name); name != "" {
			p.Allow[name] = true
		}
	}
	return p
}

// Check menilai satu baris argv.
//
// trusted menandakan argv itu berasal dari spec buatan tangan — tambalan
// bawaan uf atau milik pengguna — bukan dari korpus hasil transpile.
func (p Policy) Check(argv []string, trusted bool) Decision {
	if !p.Enabled {
		return deny("generator dimatikan lewat %s", EnvDisable)
	}
	if len(argv) == 0 {
		return deny("argv kosong")
	}

	// Berjalan sebagai root mengubah setiap efek samping menjadi efek samping
	// istimewa. Di server, satu Tab yang salah menjalankan perintah bisa jauh
	// lebih mahal daripada completion yang tidak muncul.
	if p.IsRoot && !p.AllowRoot {
		return deny("generator dimatikan saat berjalan sebagai root; setel %s untuk mengizinkan", EnvAllowRoot)
	}

	name := binaryName(argv[0])
	if name == "" {
		return deny("nama program tidak valid")
	}

	// Interpreter ditolak lebih dulu, bahkan bila namanya kebetulan sama
	// dengan perintah yang sedang dilengkapi. Mengizinkan "bash" saat
	// melengkapi bash akan membuka kembali jalur ["bash","-c","..."].
	//
	// Pengecualiannya adalah spec yang ditulis tangan dan ditinjau. Larangan
	// ini sebetulnya menyasar argumen yang isinya kode, dan nama biner hanyalah
	// perkiraan kasar untuk itu: "php artisan list" bukan kode, sedangkan
	// "php -r <apa pun>" jelas kode. Yang membedakan keduanya bukan binernya,
	// melainkan siapa yang menulis argv-nya. Korpus transpile tidak pernah
	// mendapat kelonggaran ini.
	if interpreters[name] && !trusted {
		return deny("%s adalah interpreter; argumennya adalah kode, bukan data", name)
	}

	// Aturan utama: uf tidak menjalankan program yang tidak sedang kamu
	// jalankan sendiri. Tab pada "git checkout" boleh memanggil git, dan
	// hanya git.
	if name == p.Command {
		return allow()
	}
	if p.Allow[name] {
		return allow()
	}

	return deny("%s bukan perintah yang sedang dilengkapi; tambahkan ke %s untuk mengizinkan", name, EnvAllow)
}

// binaryName mengambil nama program dari argv[0], membuang direktorinya
// sekaligus menolak bentuk yang mencurigakan.
func binaryName(arg0 string) string {
	if arg0 == "" || strings.ContainsAny(arg0, "\x00\n") {
		return ""
	}
	name := filepath.Base(filepath.FromSlash(arg0))
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return ""
	}
	return strings.TrimSuffix(name, ".exe")
}
