package ui

import "github.com/uf-cli/uf/internal/tty"

// Alias ke tipe tombol milik paket tty. Dipisah ke berkas sendiri supaya
// ketergantungan ui terhadap tty terlihat jelas dan mudah dilepas bila suatu
// saat dibutuhkan sumber tombol lain, misalnya untuk pengujian rekaman.
type (
	keyAlias     = tty.Key
	keyTypeAlias = tty.KeyType
)

const (
	KeyRune      = tty.KeyRune
	KeyTab       = tty.KeyTab
	KeyShiftTab  = tty.KeyShiftTab
	KeyEnter     = tty.KeyEnter
	KeyEscape    = tty.KeyEscape
	KeyBackspace = tty.KeyBackspace
	KeyUp        = tty.KeyUp
	KeyDown      = tty.KeyDown
	KeyLeft      = tty.KeyLeft
	KeyRight     = tty.KeyRight
	KeyPageUp    = tty.KeyPageUp
	KeyPageDown  = tty.KeyPageDown
	KeyCtrlC     = tty.KeyCtrlC
	KeyCtrlD     = tty.KeyCtrlD
	KeyCtrlU     = tty.KeyCtrlU
	KeyCtrlW     = tty.KeyCtrlW
	KeyUnknown   = tty.KeyUnknown
)
