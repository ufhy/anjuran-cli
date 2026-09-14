package main

import "testing"

// Satuan posisi kursor berbeda antar shell: bash memakai byte, sisanya rune.
// Kesalahan di sini tidak terlihat pada baris ASCII, dan baru muncul sebagai
// sisipan di tempat yang salah begitu ada huruf non-ASCII.
func TestToByteCursor(t *testing.T) {
	// "café" = 5 byte, 4 rune. "→" = 3 byte.
	tests := []struct {
		name   string
		line   string
		cursor int
		unit   string
		want   int
	}{
		{"ascii rune", "git com", 7, unitRune, 7},
		{"ascii byte", "git com", 7, unitByte, 7},
		{"rune setelah huruf beraksen", "café ", 5, unitRune, 6},
		{"rune di tengah kata beraksen", "café", 3, unitRune, 3},
		{"rune tepat sesudah é", "café", 4, unitRune, 5},
		{"byte dipakai apa adanya", "café", 5, unitByte, 5},
		{"panah unicode", "a → b", 3, unitRune, 5},
		{"kursor negatif jatuh ke akhir", "abc", -1, unitRune, 3},
		{"kursor melebihi baris", "abc", 99, unitRune, 3},
		{"byte melebihi baris", "abc", 99, unitByte, 3},
		{"baris kosong", "", 0, unitRune, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toByteCursor(tt.line, tt.cursor, tt.unit); got != tt.want {
				t.Errorf("toByteCursor(%q, %d, %s) = %d, mau %d",
					tt.line, tt.cursor, tt.unit, got, tt.want)
			}
		})
	}
}

// Satuan keluaran harus sama dengan satuan masukan, kalau tidak shell akan
// meletakkan kursornya di tempat lain daripada yang dimaksud.
func TestKonversiKursorBolakBalik(t *testing.T) {
	lines := []string{"git commit", "café au lait", "a → b → c", "日本語 test"}
	for _, line := range lines {
		runes := []rune(line)
		for i := 0; i <= len(runes); i++ {
			b := toByteCursor(line, i, unitRune)
			back := len([]rune(line[:b]))
			if back != i {
				t.Errorf("%q: rune %d -> byte %d -> rune %d", line, i, b, back)
			}
		}
	}
}

func TestSimpleModeMengikutiTerm(t *testing.T) {
	tests := []struct {
		term, simple string
		want         bool
	}{
		{"xterm-256color", "", false},
		{"screen", "", false},
		{"dumb", "", true},
		{"vt100", "", true},
		{"VT102", "", true},
		{"", "", true},
		{"xterm-256color", "1", true},
	}
	for _, tt := range tests {
		t.Setenv("TERM", tt.term)
		t.Setenv("UF_SIMPLE", tt.simple)
		if got := simpleMode(); got != tt.want {
			t.Errorf("TERM=%q UF_SIMPLE=%q -> %v, mau %v", tt.term, tt.simple, got, tt.want)
		}
	}
}

// PSReadLine memakai indeks string .NET, yaitu UTF-16 code unit. Untuk huruf
// di dalam BMP nilainya sama dengan jumlah rune, tetapi emoji dan karakter
// di luar BMP memakan DUA code unit — di sanalah rune dan utf16 berpisah,
// sementara byte sudah berpisah jauh sebelumnya.
func TestSatuanUTF16(t *testing.T) {
	// "🚀" = 4 byte, 1 rune, 2 code unit UTF-16.
	const roket = "\U0001F680"
	line := "git " + roket + " x"

	if got, want := len(line), 4+4+2; got != want {
		t.Fatalf("prasyarat: panjang byte = %d, mau %d", got, want)
	}

	tests := []struct {
		unit   string
		cursor int
		want   int // offset byte
	}{
		{unitByte, 4, 4},
		{unitRune, 4, 4},
		{unitUTF16, 4, 4}, // sebelum roket ketiganya sama

		{unitRune, 5, 8},  // sesudah roket: 1 rune
		{unitUTF16, 6, 8}, // sesudah roket: 2 code unit
		{unitByte, 8, 8},  // sesudah roket: 4 byte

		{unitUTF16, 5, 4}, // di tengah pasangan surrogate: jatuh ke batas rune
	}
	for _, tt := range tests {
		if got := toByteCursor(line, tt.cursor, tt.unit); got != tt.want {
			t.Errorf("toByteCursor(%q, %d, %s) = %d, mau %d", line, tt.cursor, tt.unit, got, tt.want)
		}
	}
}

// Konversi harus bolak-balik untuk ketiga satuan, kalau tidak kursor akan
// mendarat di tempat lain daripada yang dimaksud shell.
func TestKonversiBolakBalikSemuaSatuan(t *testing.T) {
	lines := []string{
		"git commit",
		"café au lait",
		"日本語 test",
		"deploy \U0001F680 now",
		"\U0001F1EE\U0001F1E9 bendera",
	}
	for _, line := range lines {
		for _, unit := range []string{unitRune, unitByte, unitUTF16} {
			// Telusuri setiap batas rune, karena hanya di situ kursor shell
			// bisa benar-benar berada.
			for i := range line {
				want := fromByteCursor(line, i, unit)
				got := fromByteCursor(line, toByteCursor(line, want, unit), unit)
				if got != want {
					t.Errorf("%q satuan %s: byte %d -> %d -> byte -> %d", line, unit, i, want, got)
				}
			}
		}
	}
}

func TestUTF16Len(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'a', 1},
		{'é', 1},
		{'日', 1},
		{'￿', 1},
		{'\U00010000', 2},
		{'\U0001F680', 2},
	}
	for _, tt := range tests {
		if got := utf16Len(tt.r); got != tt.want {
			t.Errorf("utf16Len(%q) = %d, mau %d", tt.r, got, tt.want)
		}
	}
}
