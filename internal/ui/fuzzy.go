// Package ui berisi lapisan yang berhubungan dengan tampilan: pencocokan
// fuzzy beserta peringkatnya, dan renderer dropdown berbasis ANSI.
package ui

import "strings"

// Match adalah hasil pencocokan satu kandidat terhadap kueri.
type Match struct {
	// Score makin besar makin relevan. Hanya bermakna relatif antar kandidat
	// pada kueri yang sama.
	Score int
	// Positions adalah indeks rune pada Name yang cocok, dipakai renderer
	// untuk menyorot huruf yang diketik.
	Positions []int
}

// Bobot penilaian. Angkanya dipilih agar urutannya stabil dan mudah dinalar:
// satu bonus awalan harus mengalahkan berapa pun bonus berurutan di tengah kata.
const (
	scoreMatch      = 16 // setiap huruf yang cocok
	bonusBoundary   = 30 // cocok tepat setelah -, _, ., / atau spasi
	bonusConsec     = 20 // cocok persis setelah huruf sebelumnya juga cocok
	bonusCamel      = 24 // cocok pada huruf besar di tengah kata
	bonusFirstChar  = 40 // cocok pada huruf pertama
	penaltyGapStart = -6 // membuka jarak pertama
	penaltyGapExtra = -2 // memperlebar jarak
)

// FuzzyMatch mencocokkan query sebagai subsequence dari name, tanpa membedakan
// huruf besar-kecil. Mengembalikan nil bila tidak cocok.
//
// Algoritmanya greedy dari kiri, bukan pemrograman dinamis penuh. Untuk nama
// perintah yang pendek hasilnya sama dengan optimal, dan biayanya O(n).
func FuzzyMatch(name, query string) *Match {
	if query == "" {
		return &Match{Score: 0}
	}

	nameRunes := []rune(name)
	queryRunes := []rune(strings.ToLower(query))
	lowerName := []rune(strings.ToLower(name))

	m := &Match{Positions: make([]int, 0, len(queryRunes))}
	qi := 0
	gapOpen := false
	prevMatched := -2

	for ni := 0; ni < len(nameRunes) && qi < len(queryRunes); ni++ {
		if lowerName[ni] != queryRunes[qi] {
			continue
		}

		score := scoreMatch
		switch {
		case ni == 0:
			score += bonusFirstChar
		case prevMatched == ni-1:
			score += bonusConsec
		case isBoundary(nameRunes[ni-1]):
			score += bonusBoundary
		case isLower(nameRunes[ni-1]) && isUpper(nameRunes[ni]):
			score += bonusCamel
		}

		// Jarak antara huruf yang cocok menurunkan skor, agar "gco" lebih
		// memilih "git-commit" daripada "generate-config-output".
		if prevMatched >= 0 && ni > prevMatched+1 {
			if !gapOpen {
				score += penaltyGapStart
				gapOpen = true
			} else {
				score += penaltyGapExtra
			}
		} else {
			gapOpen = false
		}

		m.Score += score
		m.Positions = append(m.Positions, ni)
		prevMatched = ni
		qi++
	}

	if qi < len(queryRunes) {
		return nil // tidak semua huruf kueri terpakai
	}

	// Nama yang lebih pendek lebih disukai bila skornya seri.
	m.Score -= len(nameRunes)
	return m
}

func isBoundary(r rune) bool {
	switch r {
	case '-', '_', '.', '/', ':', ' ', '=':
		return true
	}
	return false
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }
