package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tulis(t *testing.T, path, isi string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(isi), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHostsDariConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tulis(t, filepath.Join(home, ".ssh", "config"), `
# komentar
Host bastion web-01
  User deploy
  Port 2222

Host *.internal
  ProxyJump bastion

Host *
  ServerAliveInterval 60
`)
	got := Hosts()

	for _, want := range []string{"bastion", "web-01"} {
		if !punya(got, want) {
			t.Errorf("mau %q, dapat %v", want, got)
		}
	}
	// Pola tidak berguna sebagai kandidat: tidak bisa disambungi.
	for _, pola := range []string{"*", "*.internal"} {
		if punya(got, pola) {
			t.Errorf("pola %q seharusnya tidak ditawarkan", pola)
		}
	}
	// Direktif selain Host tidak boleh ikut terbaca.
	for _, bukan := range []string{"deploy", "2222", "60"} {
		if punya(got, bukan) {
			t.Errorf("%q bukan nama host, dapat %v", bukan, got)
		}
	}
}

func TestHostsDariKnownHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tulis(t, filepath.Join(home, ".ssh", "known_hosts"), `
web-01.internal ssh-ed25519 AAAA
[10.0.0.5]:2222 ssh-rsa BBBB
alias-a,alias-b ssh-ed25519 CCCC
|1|hashedbase64|hash= ssh-ed25519 DDDD
`)
	got := Hosts()

	for _, want := range []string{"web-01.internal", "10.0.0.5", "alias-a", "alias-b"} {
		if !punya(got, want) {
			t.Errorf("mau %q, dapat %v", want, got)
		}
	}
	// Bentuk [host]:port harus dipulihkan ke nama hostnya saja.
	if punya(got, "[10.0.0.5]:2222") {
		t.Error("bentuk berkurung seharusnya sudah dibersihkan")
	}
	// Berkas yang di-hash memang sengaja disembunyikan pemiliknya.
	for _, h := range got {
		if strings.HasPrefix(h, "|") {
			t.Errorf("entri ter-hash seharusnya dilewati: %q", h)
		}
	}
}

func TestHostsMengikutiInclude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tulis(t, filepath.Join(home, ".ssh", "config"), "Include conf.d/*.conf\nHost utama\n")
	tulis(t, filepath.Join(home, ".ssh", "conf.d", "kerja.conf"), "Host kantor-01\n")

	got := Hosts()
	for _, want := range []string{"utama", "kantor-01"} {
		if !punya(got, want) {
			t.Errorf("mau %q, dapat %v", want, got)
		}
	}
}

func TestHostsTanpaBerkasBukanError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := Hosts(); len(got) != 0 {
		t.Errorf("mau kosong, dapat %v", got)
	}
}

func TestEnv(t *testing.T) {
	t.Setenv("ANJURAN_UJI_VARIABEL", "nilai")
	got := Env()
	if !punya(got, "ANJURAN_UJI_VARIABEL") {
		t.Error("variabel lingkungan tidak terbaca")
	}
	for _, v := range got {
		if strings.Contains(v, "=") {
			t.Errorf("nilai ikut terbawa: %q", v)
		}
	}
}

// Template history dimaksudkan mengisi sebuah ARGUMEN. Mengembalikan baris
// perintah utuh menghasilkan kandidat yang tidak mungkin dipakai.
func TestHistoryArgsHanyaArgumen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	hist := filepath.Join(home, "hist")
	t.Setenv("HISTFILE", hist)

	tulis(t, hist, `: 1700000000:0;ssh deploy@web-01
docker compose -f x.yml up
: 1700000001:0;ssh -p 2222 root@db-01
ssh deploy@web-01
`)
	got := HistoryArgs("ssh")

	for _, want := range []string{"deploy@web-01", "root@db-01"} {
		if !punya(got, want) {
			t.Errorf("mau %q, dapat %v", want, got)
		}
	}
	// Baris milik perintah lain tidak boleh ikut.
	for _, bukan := range []string{"docker", "compose", "x.yml", "up"} {
		if punya(got, bukan) {
			t.Errorf("argumen perintah lain ikut terbawa: %q", bukan)
		}
	}
	// Opsi bukan nilai argumen; spec sudah menyediakannya sendiri.
	if punya(got, "-p") {
		t.Error("opsi seharusnya dilewati")
	}
	// Stempel waktu format zsh harus sudah dibersihkan.
	for _, g := range got {
		if strings.HasPrefix(g, ":") {
			t.Errorf("stempel waktu ikut terbawa: %q", g)
		}
	}
	// Tidak ada duplikat.
	lihat := map[string]bool{}
	for _, g := range got {
		if lihat[g] {
			t.Errorf("duplikat: %q", g)
		}
		lihat[g] = true
	}
}

func TestHistoryArgsTanpaPerintahKosong(t *testing.T) {
	if got := HistoryArgs(""); got != nil {
		t.Errorf("mau nil, dapat %v", got)
	}
}

func TestTemplateBawaanDikenali(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANJURAN_UJI_TEMPLATE", "1")

	got := FromTemplates([]string{TemplateEnv}, "", "", "")
	if !punya(got, "ANJURAN_UJI_TEMPLATE") {
		t.Errorf("template %s tidak dikerjakan", TemplateEnv)
	}
}

func punya(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
