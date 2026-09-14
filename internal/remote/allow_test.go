package remote

import (
	"os"
	"path/filepath"
	"testing"
)

func daftarHost(t *testing.T, isi string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(p, []byte(isi), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAllowlist(t *testing.T) {
	file := daftarHost(t, `
# Host yang sudah disetujui untuk pemasangan terskrip
web-01
web-*.internal
deploy@bastion
`)

	tests := []struct {
		host string
		want bool
	}{
		{"web-01", true},
		{"web-02.internal", true},
		{"deploy@bastion", true},

		// Pola dicocokkan juga ke bagian host saja, sehingga "web-01"
		// berlaku untuk pengguna mana pun.
		{"root@web-01", true},

		{"web-01.eksternal", false},
		{"prod-db", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := Allowed(tt.host, file); got != tt.want {
			t.Errorf("Allowed(%q) = %v, mau %v", tt.host, got, tt.want)
		}
	}
}

// Tanpa berkas daftar, tidak ada host yang dianggap disetujui. Ketiadaan
// konfigurasi tidak boleh berarti izin.
func TestTanpaBerkasTidakAdaYangDiizinkan(t *testing.T) {
	if Allowed("web-01", filepath.Join(t.TempDir(), "tidak-ada")) {
		t.Error("berkas yang tidak ada seharusnya tidak mengizinkan apa pun")
	}
}

func TestKomentarDanBarisKosongDiabaikan(t *testing.T) {
	file := daftarHost(t, "\n\n# web-01\n\n   \n")
	if Allowed("web-01", file) {
		t.Error("host di dalam komentar seharusnya tidak dianggap disetujui")
	}
}
