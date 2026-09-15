package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uf-cli/uf/internal/engine"
	"github.com/uf-cli/uf/internal/spec"
)

// Uji cakupan.
//
// Berbeda dari uji lain di repo ini, yang di sini menjalankan JALUR PENUH —
// engine, spec sungguhan, tambalan, template, dan generator — lalu memeriksa
// bahwa perintah yang benar-benar dipakai orang menghasilkan sesuatu.
//
// Alasannya lahir dari kegagalan berulang: setiap bug yang dilaporkan pengguna
// lolos dari pengujian saya, karena skenarionya saya pilih sendiri dan selalu
// yang sudah saya tahu bekerja. Daftar di bawah dipilih dari perintah yang
// dipakai sehari-hari, bukan dari yang mudah lulus.

// coverageEnv menyiapkan engine lengkap seperti yang dipakai CLI.
func coverageEnv(t *testing.T) (*engine.Engine, *Source, string) {
	t.Helper()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	specs := filepath.Join(root, "specs")
	if _, err := os.Stat(specs); err != nil {
		t.Skip("direktori specs/ belum dibangun; jalankan `make specs`")
	}

	extra := filepath.Join(root, "extra")
	r := spec.NewRegistryDirs(extra, specs)
	r.Trust(extra)

	// Direktori kerja berisi campuran berkas dan folder, supaya completion
	// berkas punya sesuatu untuk ditawarkan.
	return engine.New(r), &Source{Dir: root}, root
}

// candidates menjalankan jalur penuh dan mengembalikan nama kandidatnya.
func candidates(t *testing.T, eng *engine.Engine, src *Source, line string) []string {
	t.Helper()
	res, err := eng.Complete(line, len(line))
	if err != nil {
		t.Fatalf("Complete(%q): %v", line, err)
	}
	all := append([]engine.Candidate(nil), res.Candidates...)
	all = append(all, src.Candidates(res)...)

	out := make([]string, len(all))
	for i, c := range all {
		out[i] = c.Name
	}
	return out
}

// TestCakupanPerintahUmum memastikan perintah sehari-hari tidak diam.
//
// Yang diperiksa hanya "menghasilkan sesuatu", bukan isinya: isi bergantung
// pada mesin tempat uji berjalan, sedangkan DIAM adalah kegagalan di mana pun.
func TestCakupanPerintahUmum(t *testing.T) {
	eng, src, _ := coverageEnv(t)

	lines := []string{
		// Berkas dan direktori
		"cat ", "less ", "head ", "tail ", "cp ", "mv ", "rm ", "ln -s ",
		"chmod 644 ", "chown root ", "du ", "df ", "stat ", "file ", "wc ",
		"vim ", "nano ", "code ", "open ",
		"tar -c -f ", "gzip ", "unzip ", "zip ",
		"python ", "node ", "php ", "ruby ", "perl ",
		"source ", "diff ", "patch ",

		// Navigasi
		"cd ", "ls ", "mkdir ", "rmdir ", "pushd ",

		// Jaringan dan remote
		"ssh ", "scp ", "sftp ", "curl ", "wget ", "rsync ", "ping ", "dig ",
		"nc ", "openssl ",

		// Pengembangan
		"git ", "git commit ", "git checkout ", "git log ",
		"docker ", "docker run ", "docker build ",
		"kubectl ", "kubectl get ", "kubectl apply ",
		"composer ", "npm ", "yarn ", "pnpm ", "make ", "go ", "cargo ",
		"terraform ", "ansible ", "ansible-playbook ", "helm ", "aws ",

		// Sistem
		"ps ", "kill ", "top ", "grep pola ", "sed ", "awk ", "find ",
		"export ", "env ", "which ", "man ", "sudo ", "systemctl ",
		"journalctl ", "brew ", "jq ",
	}

	var diam []string
	for _, line := range lines {
		if len(candidates(t, eng, src, line)) == 0 {
			diam = append(diam, line)
		}
	}
	if len(diam) > 0 {
		t.Errorf("%d perintah tidak menghasilkan apa pun:\n  %s",
			len(diam), strings.Join(diam, "\n  "))
	}
}

// TestCakupanOpsi memastikan daftar opsi tersedia untuk perintah umum.
func TestCakupanOpsi(t *testing.T) {
	eng, src, _ := coverageEnv(t)

	// npm, go, dan journalctl sengaja tidak di sini: spec Fig meletakkan
	// opsinya di tingkat subcommand, atau memang tidak punya spec sama sekali.
	// Mendaftarkannya berarti menguji kekurangan korpus, bukan kode kita.
	cmds := []string{
		"cat", "ls", "cp", "rm", "grep", "find", "tar", "curl", "ssh", "rsync",
		"git", "docker", "kubectl", "make", "php", "composer",
		"systemctl", "ps", "sed", "jq", "terraform",
	}
	var diam []string
	for _, c := range cmds {
		if len(candidates(t, eng, src, c+" -")) == 0 {
			diam = append(diam, c)
		}
	}
	if len(diam) > 0 {
		t.Errorf("%d perintah tidak punya daftar opsi: %s", len(diam), strings.Join(diam, ", "))
	}
}

// TestCakupanBentukPath menutup kelas bug yang paling sering muncul:
// completion berkas yang diam pada bentuk path tertentu.
func TestCakupanBentukPath(t *testing.T) {
	eng, src, root := coverageEnv(t)

	home, _ := os.UserHomeDir()
	tests := []struct {
		line string
		want string // salah satu kandidat yang harus ada
	}{
		{"cat ", "README.md"},
		{"cat REA", "README.md"},
		{"cat ./REA", "./README.md"},
		{"cat internal/", "internal/engine/"},
		{"cat internal/eng", "internal/engine/"},
		{"cat ../", ""},
		{"cat /", "/etc/"},
		{"cat /etc/hos", "/etc/hosts"},
		{"cd ", "internal/"},
		{"cd internal/", "internal/engine/"},
	}
	if home != "" {
		tests = append(tests,
			struct {
				line string
				want string
			}{"cat ~", "~/"},
			struct {
				line string
				want string
			}{"cat ~/", ""},
		)
	}

	for _, tt := range tests {
		got := candidates(t, eng, src, tt.line)
		if len(got) == 0 {
			t.Errorf("%q tidak menghasilkan apa pun", tt.line)
			continue
		}
		if tt.want != "" && !has(got, tt.want) {
			t.Errorf("%q tidak memuat %q; contoh: %v", tt.line, tt.want, sample3(got))
		}
	}
	_ = root
}

// TestCakupanArgumenDinamis memeriksa perintah yang argumennya berasal dari
// tambalan buatan tangan.
func TestCakupanArgumenDinamis(t *testing.T) {
	eng, src, _ := coverageEnv(t)

	tests := []struct {
		line string
		want string
	}{
		{"kubectl get ", "pods"},
		{"kubectl get ", "services"},
		{"kubectl describe ", ""},
		// artisan diuji terpisah: ia bersyarat pada adanya berkas artisan.
		{"php ", ""},
		{"php -S ", "localhost:8000"},
		{"composer ", ""},
		{"export ", "PATH"},
		{"unset ", "PATH"},
	}
	for _, tt := range tests {
		got := candidates(t, eng, src, tt.line)
		if len(got) == 0 {
			t.Errorf("%q tidak menghasilkan apa pun", tt.line)
			continue
		}
		if tt.want != "" && !has(got, tt.want) {
			t.Errorf("%q tidak memuat %q; contoh: %v", tt.line, tt.want, sample3(got))
		}
	}
}

// TestSpecKosongYangDiketahui mendaftarkan perintah yang memang kosong karena
// spec Fig menyusunnya lewat JavaScript.
//
// Daftar ini ada supaya kekosongannya TERLIHAT dan menyusut, bukan supaya
// dimaklumi. Berkurangnya isi daftar ini adalah kemajuan yang bisa diukur.
func TestSpecKosongYangDiketahui(t *testing.T) {
	eng, src, _ := coverageEnv(t)

	// Sejak perintah tanpa argumen ikut melengkapi berkas, tidak ada lagi
	// perintah yang benar-benar diam. Kekosongan yang tersisa lebih halus:
	// perintahnya menawarkan berkas, padahal yang dibutuhkan adalah daftar
	// subcommand-nya sendiri.
	//
	// Cara mengenalinya tepat, bukan kira-kira: bandingkan dengan keluaran
	// untuk perintah yang memang tidak dikenal sama sekali. Kalau sama persis,
	// spec-nya tidak memberi apa pun.
	dasar := strings.Join(candidates(t, eng, src, "perintah-yang-pasti-tidak-ada "), "\n")

	for _, c := range []string{"php", "composer"} {
		if strings.Join(candidates(t, eng, src, c+" "), "\n") == dasar {
			t.Errorf("%s sudah ditambal tetapi isinya sama dengan perintah tak dikenal", c)
		}
	}

	// Daftar ini ada supaya kekosongannya TERLIHAT dan menyusut, bukan supaya
	// dimaklumi. Berkurangnya isi daftar adalah kemajuan yang bisa diukur.
	for _, c := range []string{"drush", "magento", "kamal", "mask", "rails"} {
		if strings.Join(candidates(t, eng, src, c+" "), "\n") != dasar {
			t.Logf("%s sudah punya isi sendiri; keluarkan dari daftar", c)
		}
	}
}

func has(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func sample3(list []string) []string {
	if len(list) > 3 {
		return list[:3]
	}
	return list
}

// Perintah tanpa spec tetap harus melengkapi nama berkas. Korpus Fig memuat
// 716 perintah; sisanya — perintah sistem, skrip internal, apa pun di PATH —
// tidak ada di sana, dan diam total di situ salah.
func TestPerintahTanpaSpecTetapMelengkapiBerkas(t *testing.T) {
	eng, src, _ := coverageEnv(t)

	for _, line := range []string{
		"gzip ", "awk ", "openssl ", "perl ", "patch ",
		"perintah-yang-tidak-ada-di-korpus ",
		"./skrip-internal.sh ",
	} {
		got := candidates(t, eng, src, line)
		if len(got) == 0 {
			t.Errorf("%q tidak menghasilkan apa pun", line)
			continue
		}
		if !has(got, "README.md") {
			t.Errorf("%q tidak melengkapi berkas; contoh: %v", line, sample3(got))
		}
	}
}

// Syarat whenFile mencegah daftar berbohong tentang apa yang bisa dijalankan:
// "php artisan" hanya ada di proyek Laravel.
func TestSyaratWhenFile(t *testing.T) {
	eng, src, _ := coverageEnv(t)

	// Repo ini bukan proyek Laravel.
	if has(candidates(t, eng, src, "php "), "artisan") {
		t.Error("artisan ditawarkan padahal berkasnya tidak ada")
	}

	// Direktori yang punya berkas artisan.
	laravel := t.TempDir()
	if err := os.WriteFile(filepath.Join(laravel, "artisan"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	eng2 := eng.InDir(laravel)
	src2 := &Source{Dir: laravel}
	if !has(candidates(t, eng2, src2, "php "), "artisan") {
		t.Error("artisan tidak ditawarkan padahal berkasnya ada")
	}
}
