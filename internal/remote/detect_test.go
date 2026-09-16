package remote

import "testing"

func TestParseUname(t *testing.T) {
	tests := []struct {
		out     string
		want    Platform
		wantErr bool
	}{
		{"Linux x86_64\n", Platform{"linux", "amd64"}, false},
		{"Linux aarch64\n", Platform{"linux", "arm64"}, false},
		{"Darwin arm64\n", Platform{"darwin", "arm64"}, false},
		{"Darwin x86_64", Platform{"darwin", "amd64"}, false},
		{"FreeBSD amd64", Platform{"freebsd", "amd64"}, false},
		{"MINGW64_NT-10.0 x86_64", Platform{"windows", "amd64"}, false},

		{"Linux armv7l", Platform{}, true},
		{"Linux i686", Platform{}, true},
		{"SunOS sun4v", Platform{}, true},
		{"Linux", Platform{}, true},
		{"", Platform{}, true},
	}

	for _, tt := range tests {
		got, err := parseUname(tt.out)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseUname(%q) err = %v, mau error = %v", tt.out, err, tt.wantErr)
			continue
		}
		if err == nil && got != tt.want {
			t.Errorf("parseUname(%q) = %v, mau %v", tt.out, got, tt.want)
		}
	}
}

// Pesan kesalahan untuk platform yang tidak didukung harus menyebut nilainya,
// karena itulah satu-satunya petunjuk yang dimiliki pengguna.
func TestPesanPlatformTakDidukung(t *testing.T) {
	_, err := parseUname("SunOS sun4v")
	if err == nil {
		t.Fatal("mau error")
	}
	if !contains(err.Error(), "SunOS") {
		t.Errorf("pesan harus menyebut sistemnya: %v", err)
	}
}

func TestQuoteMenanganiKutipTunggal(t *testing.T) {
	got := quote("/home/o'brien/bin/anjuran")
	want := `'/home/o'\''brien/bin/anjuran'`
	if got != want {
		t.Errorf("quote = %s, mau %s", got, want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// homePath harus membiarkan $HOME diekspansi shell remote sambil menjaga
// sisanya tetap harfiah. Pernah salah: seluruh path dikutip tunggal, sehingga
// berkas mendarat di direktori bernama harfiah "$HOME".
func TestHomePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{".local/bin/anjuran", `"$HOME/.local/bin/anjuran"`},
		{"/.local/bin", `"$HOME/.local/bin"`},
		{"dir dengan spasi", `"$HOME/dir dengan spasi"`},
		{`dir"kutip`, `"$HOME/dir\"kutip"`},
		{"dir$VAR", `"$HOME/dir\$VAR"`},
		{"dir`cmd`", "\"$HOME/dir\\`cmd\\`\""},
		{`dir\backslash`, `"$HOME/dir\\backslash"`},
	}
	for _, tt := range tests {
		if got := homePath(tt.in); got != tt.want {
			t.Errorf("homePath(%q) = %s, mau %s", tt.in, got, tt.want)
		}
	}
}
