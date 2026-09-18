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

// tutupMasukan menutup handle masukan — kecuali di Windows.
//
// Goroutine pembaca hampir selalu sedang menunggu tombol berikutnya saat sesi
// berakhir, dan pembacaan konsol Windows TIDAK BISA DIBATALKAN. os.File.Close
// menunggu pembacaan yang tertunda selesai, sehingga menutupnya di sini
// menggantung sampai pengguna menekan tombol lain — sesudah ia memilih
// kandidat dan mengira pekerjaannya sudah selesai.
//
// Handle-nya dibiarkan; proses widget hidup untuk satu penekanan tombol lalu
// keluar, dan Windows menutupnya sendiri saat itu. Mode konsolnya sendiri
// tetap dipulihkan lebih dulu, dan itulah yang benar-benar penting.
//
// Di Unix hal ini tidak terjadi: runtime Go bisa membangunkan pembacaan yang
// tertunda saat berkasnya ditutup.
func tutupMasukan(*os.File) error { return nil }

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
