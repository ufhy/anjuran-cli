package ui

// Pembantu untuk pengujian di paket lain.
//
// Penyapuan korpus berada di paket generator, karena di sanalah spec sungguhan
// dimuat. Dua fungsi ini membuka bagian yang perlu diperiksa tanpa membuka
// seluruh isi renderer.

// ComposeForTest menyusun baris kotak untuk sekumpulan item.
func ComposeForTest(width int, items []Item, selected, position, total int) []string {
	out := NewRenderer(nil, width, 24, false).compose(items, selected, position, total)
	for i, l := range out {
		out[i] = stripStyles(l)
	}
	return out
}

// DisplayWidth mengembalikan lebar teks dalam kolom terminal.
func DisplayWidth(s string) int { return textWidth(stripStyles(s)) }
