//go:build windows

package tty

import (
	"os"

	"golang.org/x/sys/windows"
)

// openDevice membuka konsol Windows. CONIN$ dan CONOUT$ adalah padanan
// /dev/tty: keduanya menunjuk konsol nyata meski stdio dialihkan.
func openDevice() (*os.File, *os.File, error) {
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		in.Close()
		return nil, nil, err
	}
	return in, out, nil
}

// enableVirtualTerminal menyalakan pemrosesan escape sequence pada konsol.
// Windows Terminal sudah menyalakannya sendiri, conhost lama belum. Kegagalan
// di sini tidak fatal: renderer akan tetap menulis, hanya tanpa warna.
func enableVirtualTerminal(f *os.File) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return
	}
	windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
