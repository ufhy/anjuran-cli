package generator

import (
	"strings"
	"unicode"
)

// Candidate adalah satu baris keluaran generator yang sudah dinormalkan.
type Candidate struct {
	Name        string
	Description string
}

// normalize mengubah satu baris keluaran perintah menjadi kandidat.
//
// Lapisan ini ada karena `postProcess` milik Fig — berupa closure JavaScript —
// tidak bisa dibawa saat transpile. Tanpa penggantinya, `git branch` akan
// menawarkan "* main" dan `git remote -v` menawarkan seluruh baris beserta
// URL-nya: teks yang tidak mungkin disisipkan ke baris perintah.
//
// Aturannya sengaja sedikit dan eksplisit, bukan cerdas:
//
//  1. Tab atau dua spasi atau lebih memisahkan nama dari keterangannya. Itu
//     bentuk keluaran berkolom yang paling lazim.
//  2. Penanda "* " di awal dibuang. Ini memang khusus git, dan ditulis khusus
//     karena git adalah sumber generator terbanyak dalam paket spec.
//  3. Nama yang masih mengandung spasi dibuang seluruhnya. Kandidat yang tidak
//     bisa disisipkan apa adanya lebih buruk daripada tidak ada kandidat:
//     ia menghasilkan perintah yang rusak, bukan sekadar tampilan yang jelek.
func normalize(line string) (Candidate, bool) {
	line = strings.TrimRight(line, "\r")
	line = strings.TrimRightFunc(line, unicode.IsSpace)

	// Penanda branch aktif milik git.
	if trimmed := strings.TrimLeft(line, " \t"); strings.HasPrefix(trimmed, "* ") {
		line = trimmed[2:]
	}
	line = strings.TrimLeftFunc(line, unicode.IsSpace)
	if line == "" {
		return Candidate{}, false
	}

	name, desc := splitColumns(line)
	if name == "" || !insertable(name) {
		return Candidate{}, false
	}
	return Candidate{Name: name, Description: desc}, true
}

// splitColumns memisahkan kolom pertama dari sisanya.
func splitColumns(line string) (name, desc string) {
	if i := strings.IndexByte(line, '\t'); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	if i := strings.Index(line, "  "); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i:])
	}
	return line, ""
}

// insertable menolak teks yang tidak bisa disisipkan apa adanya ke baris
// perintah.
func insertable(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	// Baris yang seluruhnya tanda baca biasanya adalah sisa pembatas tabel,
	// bukan kandidat.
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
