package ui

import "unicode"

// Lebar tampilan sebuah teks di terminal.
//
// Jumlah rune BUKAN lebar kolom. Huruf CJK dan sebagian besar emoji memakan
// dua kolom, sedangkan tanda gabung tidak memakan kolom sama sekali. Memakai
// jumlah rune untuk merapikan kolom membuat bingkai kotak patah begitu ada
// satu nama berkas berbahasa Jepang atau satu emoji di dalam keterangan.

// wide adalah rentang yang lebarnya dua kolom, mengikuti East Asian Width
// golongan W dan F beserta blok emoji yang digambar selebar dua kolom.
var wide = [][2]rune{
	{0x1100, 0x115F},   // Hangul Jamo
	{0x2E80, 0x303E},   // radikal CJK sampai tanda baca CJK
	{0x3041, 0x33FF},   // hiragana, katakana, bopomofo, kompatibilitas CJK
	{0x3400, 0x4DBF},   // ideograf CJK ekstensi A
	{0x4E00, 0x9FFF},   // ideograf CJK
	{0xA000, 0xA4CF},   // Yi
	{0xA960, 0xA97F},   // Hangul Jamo diperluas A
	{0xAC00, 0xD7A3},   // suku kata Hangul
	{0xF900, 0xFAFF},   // ideograf kompatibilitas CJK
	{0xFE10, 0xFE19},   // bentuk tegak
	{0xFE30, 0xFE6F},   // bentuk kompatibilitas CJK
	{0xFF00, 0xFF60},   // bentuk lebar penuh
	{0xFFE0, 0xFFE6},   // tanda lebar penuh
	{0x1F300, 0x1F64F}, // simbol dan piktograf, emotikon
	{0x1F680, 0x1F6FF}, // transportasi dan peta
	{0x1F900, 0x1F9FF}, // simbol tambahan dan orang
	{0x1FA70, 0x1FAFF}, // simbol diperluas A
	{0x20000, 0x3FFFD}, // ideograf CJK ekstensi B ke atas
}

// runeWidth mengembalikan jumlah kolom yang dipakai sebuah rune.
func runeWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20 || (r >= 0x7F && r < 0xA0):
		// Karakter kendali tidak menggambar apa pun. Teks yang akan ditampilkan
		// sudah dibersihkan lebih dulu, jadi ini hanya jaring pengaman.
		return 0
	case r == 0x200B || r == 0x200C || r == 0x200D || r == 0xFEFF:
		// Spasi nol, penyambung, dan penanda urutan byte.
		return 0
	case unicode.In(r, unicode.Mn, unicode.Me):
		// Tanda gabung menempel pada huruf sebelumnya.
		return 0
	}

	for _, rg := range wide {
		if r >= rg[0] && r <= rg[1] {
			return 2
		}
		if r < rg[0] {
			break // tabel terurut
		}
	}
	return 1
}

// textWidth mengembalikan lebar kolom sebuah teks.
func textWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// truncateWidth memotong teks agar muat dalam w kolom, menambahkan elipsis
// bila ada yang terpotong.
//
// Pemotongan dilakukan per rune tetapi dihitung per kolom, sehingga sebuah
// huruf lebar tidak pernah terbelah dua.
func truncateWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if textWidth(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}

	// Sisakan satu kolom untuk elipsis.
	limit := w - 1
	used := 0
	for i, r := range s {
		rw := runeWidth(r)
		if used+rw > limit {
			return s[:i] + "…"
		}
		used += rw
	}
	return s + "…"
}
