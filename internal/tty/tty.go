// Package tty membungkus akses langsung ke terminal: mode raw, ukuran layar,
// dan pembacaan tombol.
//
// Hanya berkas open_*.go yang bergantung pada sistem operasi, dan isinya
// terbatas pada cara membuka perangkat terminal. Seluruh penguraian tombol
// di bawah ini sama persis di Linux, macOS, dan Windows.
package tty

import (
	"errors"
	"io"
	"os"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// KeyType mengelompokkan tombol yang dikenali.
type KeyType int

const (
	KeyRune KeyType = iota
	KeyTab
	KeyShiftTab
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPageUp
	KeyPageDown
	KeyCtrlC
	KeyCtrlD
	KeyCtrlU
	KeyCtrlW
	// KeyHome dan KeyEnd dikenali supaya anjuran bisa MENGERJAKANNYA sendiri,
	// bukan mengembalikannya ke shell. Hanya zsh punya cara menerima tombol
	// yang dikembalikan (`zle -U`); readline, fish, dan PSReadLine tidak. Kalau
	// tombolnya tidak dikenali di sini, ia tertelan di tiga shell dari empat.
	KeyHome
	KeyEnd
	KeyUnknown
)

// Key adalah satu tombol hasil pembacaan.
type Key struct {
	Type KeyType
	Rune rune
	// Raw adalah byte asli yang membentuk tombol ini.
	//
	// Disimpan supaya tombol yang tidak ditangani bisa DIKEMBALIKAN ke shell
	// alih-alih tertelan. Tanpa itu, setiap tombol yang belum dikenali sesi
	// akan hilang tanpa jejak — dan pengguna merasakannya sebagai tombol yang
	// kadang tidak berfungsi.
	Raw []byte
}

// escTimeout adalah jeda untuk membedakan tombol Esc tunggal dari awal sebuah
// escape sequence. Nilainya harus lebih besar dari jitter jaringan pada sesi
// SSH, tetapi tetap tak terasa oleh manusia.
const escTimeout = 50 * time.Millisecond

// Terminal adalah sesi terminal dalam mode raw.
type Terminal struct {
	in    *os.File
	out   *os.File
	state *term.State

	bytes chan byte
	errs  chan error
	// raw mengumpulkan byte tombol yang sedang dibaca.
	raw []byte
}

// Open membuka terminal pengendali dan memasang mode raw.
// Pemanggil wajib memanggil Close.
func Open() (*Terminal, error) {
	in, out, err := openDevice()
	if err != nil {
		return nil, err
	}

	if !term.IsTerminal(int(in.Fd())) {
		in.Close()
		if out != in {
			out.Close()
		}
		return nil, errors.New("bukan terminal interaktif")
	}

	state, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		in.Close()
		if out != in {
			out.Close()
		}
		return nil, err
	}

	t := &Terminal{
		in:    in,
		out:   out,
		state: state,
		bytes: make(chan byte, 256),
		errs:  make(chan error, 1),
	}
	enableVirtualTerminal(out)
	go t.pump()
	return t, nil
}

// pump membaca terminal di goroutine terpisah sehingga ReadKey bisa memakai
// timeout tanpa perlu API non-blocking yang berbeda tiap sistem operasi.
func (t *Terminal) pump() {
	buf := make([]byte, 64)
	for {
		n, err := t.in.Read(buf)
		for i := 0; i < n; i++ {
			t.bytes <- buf[i]
		}
		if err != nil {
			t.errs <- err
			close(t.bytes)
			return
		}
	}
}

// Out mengembalikan writer untuk menggambar.
func (t *Terminal) Out() io.Writer { return t.out }

// Size mengembalikan lebar dan tinggi terminal saat ini.
func (t *Terminal) Size() (int, int) {
	w, h, err := term.GetSize(int(t.out.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		return 80, 24 // fallback wajar untuk terminal yang tidak melaporkan ukuran
	}
	return w, h
}

// Close memulihkan mode terminal.
func (t *Terminal) Close() error {
	err := term.Restore(int(t.in.Fd()), t.state)
	t.in.Close()
	if t.out != t.in {
		t.out.Close()
	}
	return err
}

// readByte mengambil satu byte, atau mengembalikan ok=false bila kanal tutup.
func (t *Terminal) readByte() (byte, bool) {
	b, ok := <-t.bytes
	if ok {
		t.raw = append(t.raw, b)
	}
	return b, ok
}

// readByteTimeout dipakai saat menguraikan escape sequence, di mana byte
// lanjutan mungkin memang tidak akan datang.
func (t *Terminal) readByteTimeout(d time.Duration) (byte, bool) {
	select {
	case b, ok := <-t.bytes:
		if ok {
			t.raw = append(t.raw, b)
		}
		return b, ok
	case <-time.After(d):
		return 0, false
	}
}

// Drain mengambil seluruh byte yang SUDAH terbaca tetapi belum diolah.
//
// Terminal mengirim ketikan dalam bongkahan: mengetik cepat atau menempel teks
// membuat banyak karakter tiba dalam satu pembacaan. Byte yang belum sempat
// diolah harus dikembalikan ke shell saat sesi berakhir — kalau tidak, ia
// hilang bersama proses ini, dan pengguna merasakannya sebagai karakter yang
// kadang tidak muncul.
func (t *Terminal) Drain() []byte {
	var out []byte
	for {
		select {
		case b, ok := <-t.bytes:
			if !ok {
				return out
			}
			out = append(out, b)
		default:
			return out
		}
	}
}

// ReadKey membaca satu tombol beserta byte aslinya.
func (t *Terminal) ReadKey() (Key, error) {
	t.raw = t.raw[:0]
	k, err := t.readKey()
	if err != nil {
		return k, err
	}
	k.Raw = append([]byte(nil), t.raw...)
	return k, nil
}

func (t *Terminal) readKey() (Key, error) {
	b, ok := t.readByte()
	if !ok {
		return Key{Type: KeyUnknown}, io.EOF
	}

	switch b {
	case 0x09:
		return Key{Type: KeyTab}, nil
	case 0x0d, 0x0a:
		return Key{Type: KeyEnter}, nil
	case 0x7f, 0x08:
		return Key{Type: KeyBackspace}, nil
	case 0x03:
		return Key{Type: KeyCtrlC}, nil
	case 0x04:
		return Key{Type: KeyCtrlD}, nil
	case 0x15:
		return Key{Type: KeyCtrlU}, nil
	case 0x17:
		return Key{Type: KeyCtrlW}, nil
	case 0x01:
		return Key{Type: KeyHome}, nil // Ctrl-A, kebiasaan emacs dan readline
	case 0x05:
		return Key{Type: KeyEnd}, nil // Ctrl-E
	case 0x02:
		return Key{Type: KeyLeft}, nil // Ctrl-B
	case 0x06:
		return Key{Type: KeyRight}, nil // Ctrl-F
	case 0x0e:
		return Key{Type: KeyDown}, nil // Ctrl-N, kebiasaan emacs
	case 0x10:
		return Key{Type: KeyUp}, nil // Ctrl-P
	case 0x1b:
		return t.readEscape()
	}

	if b < 0x20 {
		return Key{Type: KeyUnknown}, nil
	}
	return t.readRune(b)
}

// readEscape menguraikan sequence yang diawali ESC. Bila tidak ada byte
// lanjutan dalam escTimeout, tombol tersebut memang Esc tunggal.
func (t *Terminal) readEscape() (Key, error) {
	b, ok := t.readByteTimeout(escTimeout)
	if !ok {
		return Key{Type: KeyEscape}, nil
	}
	if b != '[' && b != 'O' {
		return Key{Type: KeyEscape}, nil
	}

	b, ok = t.readByteTimeout(escTimeout)
	if !ok {
		return Key{Type: KeyEscape}, nil
	}

	switch b {
	case 'A':
		return Key{Type: KeyUp}, nil
	case 'B':
		return Key{Type: KeyDown}, nil
	case 'C':
		return Key{Type: KeyRight}, nil
	case 'D':
		return Key{Type: KeyLeft}, nil
	case 'Z':
		return Key{Type: KeyShiftTab}, nil
	case 'H':
		return Key{Type: KeyHome}, nil // ESC [ H dan ESC O H
	case 'F':
		return Key{Type: KeyEnd}, nil // ESC [ F dan ESC O F
	case '1', '4', '5', '6', '7', '8':
		// Bentuk ESC [ N ~. Nomornya berbeda antar keluarga terminal dan
		// keduanya masih dipakai: Home bisa 1 atau 7, End bisa 4 atau 8.
		//
		// Penutupnya HARUS diperiksa, bukan sekadar dibuang. Tombol dengan
		// pengubah datang sebagai ESC [ 1 ; 5 C — awalannya sama persis, dan
		// menganggapnya Home akan menggeser kursor setiap kali seseorang
		// menekan Ctrl-panah.
		akhir, ok := t.readByteTimeout(escTimeout)
		if !ok {
			return Key{Type: KeyEscape}, nil
		}
		if akhir != '~' {
			return t.serapSisa(akhir)
		}
		switch b {
		case '5':
			return Key{Type: KeyPageUp}, nil
		case '6':
			return Key{Type: KeyPageDown}, nil
		case '1', '7':
			return Key{Type: KeyHome}, nil
		}
		return Key{Type: KeyEnd}, nil
	}

	return t.serapSisa(b)
}

// serapSisa menghabiskan sisa sebuah escape sequence yang tidak dikenali,
// supaya byte-nya tidak bocor ke prompt sebagai teks.
func (t *Terminal) serapSisa(b byte) (Key, error) {
	for b < 0x40 || b > 0x7e {
		var ok bool
		b, ok = t.readByteTimeout(escTimeout)
		if !ok {
			break
		}
	}
	return Key{Type: KeyUnknown}, nil
}

// readRune merakit satu rune UTF-8 dari byte pertama yang sudah terbaca.
func (t *Terminal) readRune(first byte) (Key, error) {
	if first < utf8.RuneSelf {
		return Key{Type: KeyRune, Rune: rune(first)}, nil
	}

	buf := []byte{first}
	for len(buf) < utf8.UTFMax {
		b, ok := t.readByteTimeout(escTimeout)
		if !ok {
			break
		}
		buf = append(buf, b)
		if r, _ := utf8.DecodeRune(buf); r != utf8.RuneError {
			return Key{Type: KeyRune, Rune: r}, nil
		}
	}
	return Key{Type: KeyUnknown}, nil
}
