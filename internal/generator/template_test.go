package generator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func pohonUji(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"catatan.txt", "data.json", ".rahasia"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{"internal", "cmd", ".git"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "engine.go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFilepaths(t *testing.T) {
	dir := pohonUji(t)
	got := Files("", dir, false)
	// Pemisahnya mengikuti sistem bila pengguna belum mengetik satu pun.
	ps := string(filepath.Separator)
	want := []string{"catatan.txt", "cmd" + ps, "data.json", "internal" + ps}
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("hasil = %v, mau %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hasil = %v, mau %v", got, want)
		}
	}
}

// Direktori diberi pemisah di ujungnya supaya penelusuran bisa langsung
// dilanjutkan — dan supaya IsDir mengenalinya sebagai direktori.
func TestDirektoriBerakhirPemisah(t *testing.T) {
	dir := pohonUji(t)
	for _, c := range Files("", dir, true) {
		if rune(c[len(c)-1]) != filepath.Separator {
			t.Errorf("direktori %q seharusnya berakhir dengan %q", c, filepath.Separator)
		}
	}
}

// Pemisah yang SEDANG DIPAKAI pengguna yang diikuti, bukan pemisah sistem.
//
// Di Windows, mengetik `cd C:\` lalu mendapat `C:\Users/` adalah jawaban
// bercampur yang tidak pernah benar untuk siapa pun. Sebaliknya, orang yang
// mengetik "/" di Windows — kebiasaan yang dibawa dari WSL atau Git Bash —
// juga berhak mendapat "/" kembali.
func TestPemisahMengikutiYangDiketik(t *testing.T) {
	dir := pohonUji(t)
	for _, p := range []string{"/", `\`} {
		hasil := Files(dir+p, "", true)
		if len(hasil) == 0 {
			t.Fatalf("awalan %q tidak menghasilkan apa-apa", dir+p)
		}
		for _, c := range hasil {
			if !strings.HasSuffix(c, p) {
				t.Errorf("awalan %q: %q tidak berakhir dengan %q", dir+p, c, p)
			}
		}
	}
}

func TestFoldersHanyaDirektori(t *testing.T) {
	dir := pohonUji(t)
	got := Files("", dir, true)
	if len(got) != 2 {
		t.Fatalf("mau 2 direktori, dapat %v", got)
	}
}

// Berkas tersembunyi hanya muncul bila memang sedang dicari; kalau tidak,
// setiap Tab di direktori rumah akan dipenuhi berkas konfigurasi.
func TestBerkasTersembunyi(t *testing.T) {
	dir := pohonUji(t)
	for _, c := range Files("", dir, false) {
		if c[0] == '.' {
			t.Errorf("berkas tersembunyi %q seharusnya tidak muncul tanpa diminta", c)
		}
	}

	got := Files(".", dir, false)
	var adaRahasia bool
	for _, c := range got {
		if c == ".rahasia" {
			adaRahasia = true
		}
	}
	if !adaRahasia {
		t.Errorf("prefix titik seharusnya memunculkan berkas tersembunyi, dapat %v", got)
	}
}

func TestPrefixMemuatDirektori(t *testing.T) {
	dir := pohonUji(t)
	got := Files("internal/eng", dir, false)
	if len(got) != 1 || got[0] != "internal/engine.go" {
		t.Fatalf("hasil = %v, mau [internal/engine.go]", got)
	}
}

func TestPrefixMenyaring(t *testing.T) {
	dir := pohonUji(t)
	got := Files("cat", dir, false)
	if len(got) != 1 || got[0] != "catatan.txt" {
		t.Fatalf("hasil = %v, mau [catatan.txt]", got)
	}
}

func TestDirektoriTidakAdaBukanError(t *testing.T) {
	if got := Files("tidak/ada/", t.TempDir(), false); got != nil {
		t.Errorf("mau nil, dapat %v", got)
	}
}

func TestFromTemplatesMenggabungkan(t *testing.T) {
	dir := pohonUji(t)
	got := FromTemplates([]string{"filepaths", "folders"}, "", dir, "")
	seen := map[string]bool{}
	for _, c := range got {
		if seen[c] {
			t.Errorf("kandidat ganda: %q", c)
		}
		seen[c] = true
	}
	if len(got) != 4 {
		t.Errorf("mau 4 kandidat unik, dapat %v", got)
	}
}

func TestTemplateTidakDikenalDiabaikan(t *testing.T) {
	if got := FromTemplates([]string{"history", "help"}, "", pohonUji(t), ""); got != nil {
		t.Errorf("template yang belum didukung seharusnya diabaikan diam-diam, dapat %v", got)
	}
}

func TestSplitPrefix(t *testing.T) {
	tests := []struct{ in, dir, base string }{
		{"", "", ""},
		{"cat", "", "cat"},
		{"internal/eng", "internal/", "eng"},
		{"a/b/c", "a/b/", "c"},
		{"/abs/path", "/abs/", "path"},
		{"dir/", "dir/", ""},
	}
	for _, tt := range tests {
		d, b := splitPrefix(tt.in)
		if d != tt.dir || b != tt.base {
			t.Errorf("splitPrefix(%q) = (%q,%q), mau (%q,%q)", tt.in, d, b, tt.dir, tt.base)
		}
	}
}
