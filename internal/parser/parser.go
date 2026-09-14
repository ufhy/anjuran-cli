// Package parser memecah satu baris shell menjadi token beserta offset-nya,
// lalu menentukan token mana yang sedang diedit oleh kursor.
//
// Parser ini sengaja murni: input berupa string + posisi kursor, output
// berupa struct. Tidak ada interaksi dengan terminal sama sekali, sehingga
// seluruh perilakunya bisa diuji tanpa PTY.
package parser

import "strings"

// Quote menandai jenis kutip yang membungkus sebuah token.
type Quote byte

const (
	QuoteNone   Quote = 0
	QuoteSingle Quote = '\''
	QuoteDouble Quote = '"'
)

// Token adalah satu kata pada baris perintah.
type Token struct {
	// Value adalah isi token setelah kutip dan escape dilepas.
	Value string
	// Start dan End adalah offset byte pada baris asli; End eksklusif.
	Start, End int
	// Quote berisi jenis kutip pembuka, QuoteNone bila tidak dikutip.
	Quote Quote
	// Terminated bernilai false bila kutipnya belum ditutup saat baris habis.
	Terminated bool
	// IsSeparator menandai token kontrol seperti |, &&, ; yang memulai
	// perintah baru.
	IsSeparator bool
}

// Line adalah hasil parse satu baris berikut posisi kursornya.
type Line struct {
	Raw    string
	Cursor int
	Tokens []Token
	// CursorIndex menunjuk token yang sedang diedit. Bernilai len(Tokens)
	// bila kursor berada di ruang kosong setelah token terakhir.
	CursorIndex int
	// Prefix adalah bagian token kursor yang berada SEBELUM kursor. Inilah
	// yang dipakai untuk memfilter kandidat.
	Prefix string
	// NewToken bernilai true bila kursor berada di whitespace, artinya
	// pengguna sedang mulai mengetik kata baru.
	NewToken bool
}

// separators adalah operator shell yang mengakhiri satu perintah.
// Diurutkan dari yang terpanjang agar "&&" tidak keburu cocok dengan "&".
var separators = []string{"&&", "||", "|", ";", "&"}

// Parse memecah line dan menentukan konteks kursor. Cursor dijepit ke
// rentang yang valid sehingga pemanggil tidak perlu memvalidasinya.
func Parse(line string, cursor int) *Line {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(line) {
		cursor = len(line)
	}

	l := &Line{Raw: line, Cursor: cursor}
	l.Tokens = tokenize(line)
	l.locateCursor()
	return l
}

func tokenize(line string) []Token {
	var tokens []Token
	i := 0
	for i < len(line) {
		// Lewati whitespace antar token.
		if line[i] == ' ' || line[i] == '\t' {
			i++
			continue
		}

		if sep, ok := matchSeparator(line[i:]); ok {
			tokens = append(tokens, Token{
				Value:       sep,
				Start:       i,
				End:         i + len(sep),
				Terminated:  true,
				IsSeparator: true,
			})
			i += len(sep)
			continue
		}

		tok, next := readWord(line, i)
		tokens = append(tokens, tok)
		i = next
	}
	return tokens
}

func matchSeparator(s string) (string, bool) {
	for _, sep := range separators {
		if strings.HasPrefix(s, sep) {
			return sep, true
		}
	}
	return "", false
}

// readWord membaca satu kata mulai dari start, menghormati kutip tunggal,
// kutip ganda, dan backslash. Mengembalikan token dan offset lanjutan.
func readWord(line string, start int) (Token, int) {
	var sb strings.Builder
	tok := Token{Start: start, Terminated: true}

	i := start
	for i < len(line) {
		c := line[i]

		switch {
		case c == ' ' || c == '\t':
			// Whitespace di luar kutip mengakhiri kata.
			tok.Value = sb.String()
			tok.End = i
			return tok, i

		case c == '\\' && i+1 < len(line):
			// Escape: karakter berikutnya diambil apa adanya.
			sb.WriteByte(line[i+1])
			i += 2

		case c == '\'' || c == '"':
			quote := c
			if tok.Quote == QuoteNone {
				tok.Quote = Quote(quote)
			}
			i++ // lewati kutip pembuka
			closed := false
			for i < len(line) {
				// Di dalam kutip tunggal, backslash tidak punya arti khusus.
				if quote == '"' && line[i] == '\\' && i+1 < len(line) {
					sb.WriteByte(line[i+1])
					i += 2
					continue
				}
				if line[i] == quote {
					i++
					closed = true
					break
				}
				sb.WriteByte(line[i])
				i++
			}
			if !closed {
				// Kutip belum ditutup: kata membentang sampai akhir baris.
				tok.Value = sb.String()
				tok.End = len(line)
				tok.Terminated = false
				return tok, len(line)
			}

		default:
			if _, isSep := matchSeparator(line[i:]); isSep {
				// Separator juga mengakhiri kata, misalnya "foo|bar".
				tok.Value = sb.String()
				tok.End = i
				return tok, i
			}
			sb.WriteByte(c)
			i++
		}
	}

	tok.Value = sb.String()
	tok.End = len(line)
	return tok, len(line)
}

// locateCursor menentukan token yang sedang diedit beserta prefix-nya.
func (l *Line) locateCursor() {
	for idx, t := range l.Tokens {
		if l.Cursor < t.Start {
			break // kursor ada di whitespace sebelum token ini
		}
		if l.Cursor <= t.End {
			if t.IsSeparator {
				// Kursor menempel pada separator: perlakukan sebagai kata baru.
				break
			}
			l.CursorIndex = idx
			l.Prefix = prefixOf(l.Raw, t, l.Cursor)
			l.NewToken = false
			return
		}
	}

	// Kursor tidak berada di dalam token mana pun: kata baru.
	l.CursorIndex = l.insertionIndex()
	l.Prefix = ""
	l.NewToken = true
}

// insertionIndex mencari posisi di mana token baru akan disisipkan.
func (l *Line) insertionIndex() int {
	for idx, t := range l.Tokens {
		if l.Cursor <= t.Start {
			return idx
		}
	}
	return len(l.Tokens)
}

// prefixOf memotong token pada posisi kursor lalu melepas kutip/escape-nya,
// sehingga "--mes|sage" menghasilkan "--mes".
func prefixOf(raw string, t Token, cursor int) string {
	partial := raw[t.Start:cursor]
	sub, _ := readWord(partial, 0)
	return sub.Value
}

// Words mengembalikan token dari perintah TERAKHIR sebelum kursor, yaitu
// setelah separator terakhir. Inilah yang dikonsumsi oleh engine, sehingga
// "docker ps | grep ng" melakukan completion terhadap grep, bukan docker.
//
// Nilai kedua adalah indeks token kursor relatif terhadap slice tersebut.
func (l *Line) Words() ([]Token, int) {
	startIdx := 0
	for idx := 0; idx < l.CursorIndex && idx < len(l.Tokens); idx++ {
		if l.Tokens[idx].IsSeparator {
			startIdx = idx + 1
		}
	}

	end := l.CursorIndex
	if end > len(l.Tokens) {
		end = len(l.Tokens)
	}
	if startIdx > end {
		startIdx = end
	}
	return l.Tokens[startIdx:end], l.CursorIndex - startIdx
}
