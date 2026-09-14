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

// enableVirtualTerminal tidak diperlukan di luar Windows.
func enableVirtualTerminal(*os.File) {}
