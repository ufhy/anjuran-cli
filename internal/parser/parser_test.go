package parser

import "testing"

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{"sederhana", "git commit -m pesan", []string{"git", "commit", "-m", "pesan"}},
		{"spasi berlebih", "  git   status  ", []string{"git", "status"}},
		{"kutip ganda", `git commit -m "pesan panjang"`, []string{"git", "commit", "-m", "pesan panjang"}},
		{"kutip tunggal", `git commit -m 'apa adanya \n'`, []string{"git", "commit", "-m", `apa adanya \n`}},
		{"escape spasi", `cat my\ file.txt`, []string{"cat", "my file.txt"}},
		{"pipe tanpa spasi", "docker ps|grep web", []string{"docker", "ps", "|", "grep", "web"}},
		{"and berantai", "make && ./run", []string{"make", "&&", "./run"}},
		{"kutip belum ditutup", `git commit -m "belum`, []string{"git", "commit", "-m", "belum"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenize(tt.line)
			if len(got) != len(tt.want) {
				t.Fatalf("jumlah token = %d (%v), mau %d (%v)", len(got), values(got), len(tt.want), tt.want)
			}
			for i := range got {
				if got[i].Value != tt.want[i] {
					t.Errorf("token[%d] = %q, mau %q", i, got[i].Value, tt.want[i])
				}
			}
		})
	}
}

func values(ts []Token) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Value
	}
	return out
}

func TestCursorContext(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		cursor       int
		wantPrefix   string
		wantNewToken bool
		wantIndex    int
	}{
		{"akhir kata", "git comm", 8, "comm", false, 1},
		{"setelah spasi", "git ", 4, "", true, 1},
		{"tengah kata", "git checkout", 7, "che", false, 1},
		{"baris kosong", "", 0, "", true, 0},
		{"di dalam kutip", `git commit -m "hal`, 18, "hal", false, 3},
		{"sebelum token", "git  status", 4, "", true, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := Parse(tt.line, tt.cursor)
			if l.Prefix != tt.wantPrefix {
				t.Errorf("Prefix = %q, mau %q", l.Prefix, tt.wantPrefix)
			}
			if l.NewToken != tt.wantNewToken {
				t.Errorf("NewToken = %v, mau %v", l.NewToken, tt.wantNewToken)
			}
			if l.CursorIndex != tt.wantIndex {
				t.Errorf("CursorIndex = %d, mau %d", l.CursorIndex, tt.wantIndex)
			}
		})
	}
}

// Words harus memulai ulang setelah separator, sehingga completion di
// "docker ps | grep ng" berlaku untuk grep.
func TestWordsResetSetelahSeparator(t *testing.T) {
	l := Parse("docker ps | grep ng", 19)
	words, rel := l.Words()
	if len(words) != 1 || words[0].Value != "grep" {
		t.Fatalf("words = %v, mau [grep]", values(words))
	}
	if rel != 1 {
		t.Errorf("indeks relatif = %d, mau 1", rel)
	}
}

func TestCursorTidakMelebihiBaris(t *testing.T) {
	l := Parse("git", 999)
	if l.Cursor != 3 {
		t.Errorf("Cursor = %d, mau dijepit ke 3", l.Cursor)
	}
}
