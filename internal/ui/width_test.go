package ui

import "testing"

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'a', 1}, {'Z', 1}, {'1', 1}, {'-', 1}, {' ', 1},
		{'é', 1}, {'ñ', 1},

		// CJK dan lebar penuh memakan dua kolom.
		{'日', 2}, {'本', 2}, {'語', 2}, {'한', 2}, {'中', 2}, {'，', 2},

		// Emoji digambar selebar dua kolom.
		{'\U0001F680', 2}, {'\U0001F525', 2}, {'\U0001F600', 2},

		// Tanda gabung menempel pada huruf sebelumnya.
		{'́', 0}, {'̀', 0},

		// Tidak menggambar apa pun.
		{rune(0x200B), 0}, {rune(0x200D), 0}, {rune(0xFEFF), 0}, {'\n', 0}, {0, 0},
	}
	for _, tt := range tests {
		if got := runeWidth(tt.r); got != tt.want {
			t.Errorf("runeWidth(%q) = %d, mau %d", tt.r, got, tt.want)
		}
	}
}

func TestTextWidth(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"biasa", 5},
		{"日本語", 6},
		{"a日b", 4},
		{"roket \U0001F680", 8},
		{"é", 1}, // e ditambah tanda gabung
	}
	for _, tt := range tests {
		if got := textWidth(tt.s); got != tt.want {
			t.Errorf("textWidth(%q) = %d, mau %d", tt.s, got, tt.want)
		}
	}
}

// Pemotongan dihitung per kolom, sehingga huruf lebar tidak pernah terbelah
// dua dan hasilnya tidak pernah melampaui lebar yang diminta.
func TestTruncateWidth(t *testing.T) {
	tests := []struct {
		s    string
		w    int
		want string
	}{
		{"biasa", 10, "biasa"},
		{"biasa", 5, "biasa"},
		{"biasa", 4, "bia…"},
		{"biasa", 1, "…"},
		{"biasa", 0, ""},
		{"日本語", 6, "日本語"},
		{"日本語", 5, "日本…"},
		{"日本語", 4, "日…"},
		{"日本語", 3, "日…"},
		{"a日本", 4, "a日…"},
	}
	for _, tt := range tests {
		got := truncateWidth(tt.s, tt.w)
		if got != tt.want {
			t.Errorf("truncateWidth(%q, %d) = %q, mau %q", tt.s, tt.w, got, tt.want)
		}
		if w := textWidth(got); w > tt.w {
			t.Errorf("truncateWidth(%q, %d) = %q selebar %d kolom", tt.s, tt.w, got, w)
		}
	}
}
