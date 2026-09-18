//go:build !windows

package tty

import "os"

// openDevice membuka terminal pengendali. /dev/tty dipakai, bukan stdin,
// karena stdout dan stdin kita dialihkan oleh shell saat memanggil widget.
func openDevice() (*os.File, *os.File, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// tutupMasukan menutup handle masukan. Di Unix penutupan membangunkan
// pembacaan yang sedang tertunda, jadi tidak ada yang menggantung.
func tutupMasukan(f *os.File) error { return f.Close() }

// enableVirtualTerminal tidak diperlukan di luar Windows.
func enableVirtualTerminal(*os.File) {}
