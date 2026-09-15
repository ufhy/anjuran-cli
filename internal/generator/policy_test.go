package generator

import "testing"

func basePolicy() Policy {
	return Policy{Command: "git", Allow: map[string]bool{}, Enabled: true}
}

func TestMengizinkanPerintahYangSedangDilengkapi(t *testing.T) {
	p := basePolicy()
	for _, argv := range [][]string{
		{"git", "branch"},
		{"/usr/bin/git", "remote"},
		{"git.exe", "tag"},
	} {
		if d := p.Check(argv, false); !d.Allowed {
			t.Errorf("Check(%v) ditolak: %s", argv, d.Reason)
		}
	}
}

func TestMenolakBinerLain(t *testing.T) {
	p := basePolicy()
	for _, argv := range [][]string{
		{"curl", "https://contoh.test"},
		{"cat", "/etc/passwd"},
		{"kubectl", "get", "pods"},
	} {
		if d := p.Check(argv, false); d.Allowed {
			t.Errorf("Check(%v) seharusnya ditolak", argv)
		}
	}
}

// Ini alasan utama daftar interpreter ada: 194 generator di paket spec Fig
// berbentuk ["bash","-c","<skrip>"], sehingga bentuk argv saja tidak
// mencegah eksekusi shell.
func TestMenolakShellLewatArgv(t *testing.T) {
	p := basePolicy()
	argv := []string{"bash", "-c", "curl https://contoh.test | sh"}
	d := p.Check(argv, false)
	if d.Allowed {
		t.Fatal("bash -c seharusnya ditolak")
	}
	if d.Reason == "" {
		t.Error("penolakan harus punya alasan yang bisa ditampilkan")
	}
}

// Bahkan saat perintah yang dilengkapi MEMANG interpreter, argumennya tetap
// berupa kode. Mengizinkannya akan membuka kembali jalur yang baru ditutup.
func TestInterpreterDitolakMeskiSamaDenganPerintah(t *testing.T) {
	for _, name := range []string{"bash", "python3", "node", "sh"} {
		p := basePolicy()
		p.Command = name
		if d := p.Check([]string{name, "-c", "print(1)"}, false); d.Allowed {
			t.Errorf("%s seharusnya ditolak meski sedang dilengkapi", name)
		}
	}
}

func TestInterpreterDitolakMeskiAdaDiAllowlist(t *testing.T) {
	p := basePolicy()
	p.Allow["bash"] = true
	if d := p.Check([]string{"bash", "-c", "id"}, false); d.Allowed {
		t.Error("allowlist tidak boleh menembus larangan interpreter")
	}
}

// Larangan interpreter sebetulnya menyasar argumen yang isinya kode, dan nama
// biner hanyalah perkiraan kasar untuk itu. Spec yang ditulis tangan dan
// ditinjau boleh melewatinya; korpus hasil transpile tidak pernah.
func TestInterpreterDiizinkanBilaSpecnyaTepercaya(t *testing.T) {
	p := basePolicy()
	p.Command = "php"

	if d := p.Check([]string{"php", "artisan", "list"}, false); d.Allowed {
		t.Error("spec tidak tepercaya seharusnya tetap ditolak")
	}
	if d := p.Check([]string{"php", "artisan", "list"}, true); !d.Allowed {
		t.Errorf("spec tepercaya seharusnya diizinkan: %s", d.Reason)
	}
}

// Kepercayaan tidak menembus aturan lain: biner yang sama sekali berbeda dari
// perintah yang diketik tetap ditolak.
func TestTepercayaTidakMenembusAturanLain(t *testing.T) {
	p := basePolicy()
	if d := p.Check([]string{"curl", "https://contoh.test"}, true); d.Allowed {
		t.Error("biner lain tetap harus ditolak meski spec-nya tepercaya")
	}

	p.IsRoot = true
	if d := p.Check([]string{"git", "branch"}, true); d.Allowed {
		t.Error("berjalan sebagai root tetap harus menolak")
	}
}

func TestSudoDanEnvDitolak(t *testing.T) {
	p := basePolicy()
	for _, name := range []string{"sudo", "doas", "su", "env", "xargs", "nohup"} {
		if d := p.Check([]string{name, "git", "branch"}, false); d.Allowed {
			t.Errorf("%s seharusnya ditolak; ia meneruskan eksekusi ke program lain", name)
		}
	}
}

func TestAllowlistMengizinkanBinerTambahan(t *testing.T) {
	p := basePolicy()
	p.Command = "tmuxinator"
	p.Allow["tmux"] = true
	if d := p.Check([]string{"tmux", "ls"}, false); !d.Allowed {
		t.Errorf("biner dalam allowlist harus diizinkan: %s", d.Reason)
	}
}

func TestMatiSaatRoot(t *testing.T) {
	p := basePolicy()
	p.IsRoot = true
	if d := p.Check([]string{"git", "branch"}, false); d.Allowed {
		t.Error("generator harus mati saat berjalan sebagai root")
	}

	p.AllowRoot = true
	if d := p.Check([]string{"git", "branch"}, false); !d.Allowed {
		t.Errorf("izin eksplisit harus dihormati: %s", d.Reason)
	}
}

func TestBisaDimatikanSeluruhnya(t *testing.T) {
	p := basePolicy()
	p.Enabled = false
	if d := p.Check([]string{"git", "branch"}, false); d.Allowed {
		t.Error("generator harus mati saat Enabled false")
	}
}

func TestArgvTidakValid(t *testing.T) {
	p := basePolicy()
	for _, argv := range [][]string{
		{},
		{""},
		{"git\x00jahat"},
		{".."},
		{"/"},
	} {
		if d := p.Check(argv, false); d.Allowed {
			t.Errorf("Check(%q) seharusnya ditolak", argv)
		}
	}
}

// Penolakan berbasis nama dasar tidak boleh bisa dilewati dengan path.
func TestPathTidakBisaMenyelundupkanInterpreter(t *testing.T) {
	p := basePolicy()
	for _, argv := range [][]string{
		{"/bin/bash", "-c", "id"},
		{"../../bin/sh", "-c", "id"},
		{"/usr/local/bin/python3", "-c", "1"},
	} {
		if d := p.Check(argv, false); d.Allowed {
			t.Errorf("Check(%v) seharusnya ditolak", argv)
		}
	}
}
