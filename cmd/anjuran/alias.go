package main

import (
	"github.com/ufhy/anjuran-cli/internal/parser"
)

// aliasExpansion menjembatani baris yang memakai alias dengan perintah
// sebenarnya.
//
// Tanpa ini, `gco fit` tidak menghasilkan apa pun: anjuran mencari spec bernama
// "gco" dan tidak menemukannya. Padahal alias justru cara sehari-hari orang
// memakai perintah panjang — oh-my-zsh sendiri memasang ratusan.
//
// Perhitungan dilakukan pada bentuk yang sudah dimekarkan, tetapi hasilnya
// dikembalikan ke baris ASLI. Menukar "gco" menjadi "git checkout" di layar
// akan mengubah apa yang pengguna ketik tanpa diminta, dan itu jauh lebih
// mengganggu daripada tidak ada completion.
type aliasExpansion struct {
	// active bernilai false bila tidak ada yang perlu dipetakan.
	active bool
	// firstEnd adalah offset byte akhir kata pertama pada baris asli.
	firstEnd int
	// expansion adalah teks pengganti kata pertama.
	expansion string
	// delta adalah selisih panjang, dipakai menggeser posisi kursor.
	delta int
	// head adalah kata pertama asli, dipasang kembali saat memulihkan.
	head string
}

// newAliasExpansion menyiapkan pemetaan untuk sebuah baris.
//
// expansion kosong, atau kursor yang masih berada DI DALAM kata pertama,
// membuat pemetaan tidak aktif: pengguna sedang mengetik nama aliasnya
// sendiri, dan memekarkannya di situ hanya akan melengkapi perintah yang
// belum tentu ia maksud.
func newAliasExpansion(line string, cursor int, expansion string) aliasExpansion {
	if expansion == "" {
		return aliasExpansion{}
	}

	l := parser.Parse(line, cursor)
	if len(l.Tokens) == 0 || l.Tokens[0].IsSeparator {
		return aliasExpansion{}
	}
	first := l.Tokens[0]
	if cursor <= first.End {
		return aliasExpansion{}
	}

	return aliasExpansion{
		active:    true,
		firstEnd:  first.End,
		expansion: expansion,
		delta:     len(expansion) - first.End,
		head:      line[:first.End],
	}
}

// Line mengembalikan baris yang dipakai untuk menghitung kandidat.
func (a aliasExpansion) Line(orig string) string {
	if !a.active {
		return orig
	}
	return a.expansion + orig[a.firstEnd:]
}

// Cursor memetakan posisi kursor dari baris asli ke baris yang dimekarkan.
func (a aliasExpansion) Cursor(c int) int {
	if !a.active {
		return c
	}
	return c + a.delta
}

// Restore memetakan hasil kembali ke baris asli.
func (a aliasExpansion) Restore(line string, cursor int) (string, int) {
	if !a.active {
		return line, cursor
	}
	// Kata pertama yang dimekarkan dipotong, lalu bentuk aslinya dipasang
	// kembali. Panjang ekspansi dipakai apa adanya karena engine tidak pernah
	// menyentuh token pertama: kursornya selalu berada sesudah token itu.
	if len(line) < len(a.expansion) {
		return line, cursor
	}
	restored := a.head + line[len(a.expansion):]

	c := cursor - a.delta
	if c < 0 {
		c = 0
	}
	if c > len(restored) {
		c = len(restored)
	}
	return restored, c
}
