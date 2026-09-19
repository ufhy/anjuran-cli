package ui

import "github.com/ufhy/anjuran-cli/internal/engine"

// Mode ikon yang dikenali.
const (
	IkonAman = "aman" // bentuk geometris yang ada di hampir semua font
	IkonNerd = "nerd" // glyph Nerd Font
	IkonMati = "mati"
)

// ikonAman memakai bentuk yang koheren, bukan gambar.
//
// Dipilih begini karena font tidak bisa ditanya: sebuah glyph yang tidak
// dimiliki font akan tergambar sebagai kotak kosong, dan lebarnya bisa meleset
// sehingga bingkai kotaknya patah. Bentuk geometris dasar ada di hampir semua
// font dan selebar satu kolom di mana pun.
//
// Artinya konsisten: padat berarti perintah, berongga berarti pengubahnya,
// panah berarti bisa ditelusuri, titik berarti sesuatu yang diam.
var ikonAman = map[engine.Kind]string{
	engine.KindSubcommand: "▪",
	engine.KindOption:     "▫",
	engine.KindArg:        "·",
}

// ikonNerd memakai glyph Nerd Font, yang jauh lebih terbaca bila fontnya ada.
var ikonNerd = map[engine.Kind]string{
	engine.KindSubcommand: "", // terminal
	engine.KindOption:     "", // roda gigi
	engine.KindArg:        "", // berkas
}

const (
	direktoriAman = "▸"
	direktoriNerd = ""
)

// ikonUntuk memilih ikon sebuah kandidat.
//
// Baris "berhenti" tidak diberi ikon: namanya SUDAH berupa ikon, dan
// menggambarnya dua kali hanya mengulang hal yang sama.
func ikonUntuk(c engine.Candidate, mode string) string {
	if mode == IkonMati || c.Kind == engine.KindBerhenti {
		return ""
	}
	if c.IsDir() {
		if mode == IkonNerd {
			return direktoriNerd
		}
		return direktoriAman
	}
	tabel := ikonAman
	if mode == IkonNerd {
		tabel = ikonNerd
	}
	if s, ok := tabel[c.Kind]; ok {
		return s
	}
	return tabel[engine.KindArg]
}
