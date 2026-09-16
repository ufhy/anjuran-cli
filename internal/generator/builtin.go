package generator

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Template bawaan tambahan milik anjuran.
//
// Diawali "anjuran:" supaya tidak pernah bertabrakan dengan nama template Fig bila
// kelak ada yang baru. Semuanya BUKAN generator: tidak satu pun menumbuhkan
// proses, sehingga tidak tunduk pada kebijakan generator dan tidak pernah
// menjadi efek samping yang mahal.
const (
	TemplateHosts = "anjuran:hosts"
	TemplateEnv   = "anjuran:env"
)

// maxLines membatasi pembacaan berkas yang bisa saja sangat besar.
const maxLines = 20000

// Hosts mengumpulkan nama host yang dikenal dari konfigurasi SSH pengguna.
//
// Dibaca dari berkas, bukan dari perintah: `ssh` tidak punya subperintah yang
// mencetak daftar host, dan menjalankan apa pun untuk ini akan berlebihan.
func Hosts() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	ssh := filepath.Join(home, ".ssh")

	seen := map[string]bool{}
	var out []string
	add := func(h string) {
		// Pola dan alamat negatif di Host tidak berguna sebagai kandidat.
		if h == "" || h == "*" || strings.ContainsAny(h, "*?!") || seen[h] {
			return
		}
		seen[h] = true
		out = append(out, h)
	}

	for _, h := range hostsFromConfig(filepath.Join(ssh, "config"), 0) {
		add(h)
	}
	for _, h := range hostsFromKnownHosts(filepath.Join(ssh, "known_hosts")) {
		add(h)
	}

	sort.Strings(out)
	return out
}

// hostsFromConfig membaca direktif Host, mengikuti Include satu tingkat.
func hostsFromConfig(path string, depth int) []string {
	if depth > 3 {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for n := 0; sc.Scan() && n < maxLines; n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch strings.ToLower(fields[0]) {
		case "host":
			out = append(out, fields[1:]...)
		case "include":
			// Include memakai path relatif terhadap ~/.ssh dan boleh berpola.
			for _, pat := range fields[1:] {
				if !filepath.IsAbs(pat) {
					pat = filepath.Join(filepath.Dir(path), pat)
				}
				matches, _ := filepath.Glob(pat)
				for _, m := range matches {
					out = append(out, hostsFromConfig(m, depth+1)...)
				}
			}
		}
	}
	return out
}

// hostsFromKnownHosts membaca kolom pertama setiap baris.
//
// Berkas yang sudah di-hash tidak menghasilkan apa-apa, dan itu memang
// disengaja oleh pemiliknya — nama host di situ sengaja disembunyikan.
func hostsFromKnownHosts(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for n := 0; sc.Scan() && n < maxLines; n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		for _, h := range strings.Split(fields[0], ",") {
			// Bentuk [host]:port dipulihkan ke nama hostnya saja.
			h = strings.TrimPrefix(h, "[")
			if i := strings.Index(h, "]"); i >= 0 {
				h = h[:i]
			}
			out = append(out, h)
		}
	}
	return out
}

// Env mengumpulkan nama variabel lingkungan.
func Env() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			out = append(out, kv[:i])
		}
	}
	sort.Strings(out)
	return out
}

// HistoryArgs mengumpulkan ARGUMEN yang pernah dipakai bersama sebuah perintah,
// terbaru lebih dulu.
//
// Bukan seluruh baris perintahnya. Template "history" milik Fig dimaksudkan
// untuk mengisi sebuah argumen — pada spec ssh, misalnya, yang dicari adalah
// host yang pernah dituju. Mengembalikan baris utuh menghasilkan kandidat
// seperti "docker compose -f x.yml build" untuk ssh: tidak mungkin dipakai,
// dan hanya menutupi kandidat yang benar.
func HistoryArgs(command string) []string {
	if command == "" {
		return nil
	}

	seen := map[string]bool{}
	var out []string
	for _, line := range history() {
		fields := strings.Fields(line)
		if len(fields) < 2 || filepath.Base(fields[0]) != command {
			continue
		}
		for _, f := range fields[1:] {
			// Opsi bukan nilai argumen, dan spec sudah menyediakannya sendiri.
			if strings.HasPrefix(f, "-") || seen[f] {
				continue
			}
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// history membaca baris perintah dari berkas riwayat shell, terbaru lebih dulu.
func history() []string {
	path := os.Getenv("HISTFILE")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		for _, name := range []string{".zsh_history", ".bash_history"} {
			p := filepath.Join(home, name)
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}
	if path == "" {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	seen := map[string]bool{}
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for n := 0; sc.Scan() && n < maxLines; n++ {
		line := strings.TrimSpace(sc.Text())
		// Format diperluas milik zsh: ": <epoch>:<durasi>;<perintah>"
		if strings.HasPrefix(line, ": ") {
			if i := strings.IndexByte(line, ';'); i >= 0 {
				line = line[i+1:]
			}
		}
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}

	// Yang terbaru ada di akhir berkas; dibalik supaya paling relevan di atas.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
