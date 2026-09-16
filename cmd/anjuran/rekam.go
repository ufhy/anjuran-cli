package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// EnvRekam menyalakan perekaman byte yang digambar ke terminal.
const EnvRekam = "ANJURAN_LOG"

// rekam membungkus tujuan gambar agar setiap byte yang dikirim ke terminal
// ikut disalin ke sebuah berkas.
//
// Ada untuk satu keperluan: menyelidiki laporan "layarnya aneh" di terminal
// orang lain. Gejala seperti itu tidak selalu bisa ditiru — konfigurasi shell,
// tema prompt, ukuran jendela, dan alat lain yang ikut menggambar semuanya
// ikut menentukan. Meminta pengguna merekam dengan `script` menambah satu
// lapisan PTY lagi, dan lapisan itu sendiri bisa mengubah gejalanya.
//
// Yang direkam hanya yang DITULIS anjuran; apa yang ditulis shell tidak
// terlihat di sini. Itu justru yang diinginkan: ia menjawab "apakah anjuran
// yang menghapusnya" tanpa memuat apa pun dari layar pengguna selain gambar
// kotaknya sendiri.
// Nilai kembalian kedua menutup berkasnya, dan harus selalu dipanggil.
//
// Membiarkannya terbuka sampai proses keluar tampak aman di Unix, tetapi di
// Windows berkas yang masih dipegang tidak bisa dihapus atau dipindahkan oleh
// siapa pun — termasuk oleh pengguna yang ingin membersihkan rekamannya.
func rekam(w io.Writer) (io.Writer, func()) {
	path := os.Getenv(EnvRekam)
	if path == "" {
		return w, func() {}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		// Perekaman adalah alat bantu; kegagalannya tidak boleh menghalangi
		// pekerjaan yang sebenarnya.
		return w, func() {}
	}
	fmt.Fprintf(f, "\n--- %s pid=%d ---\n", time.Now().Format("15:04:05.000"), os.Getpid())
	return io.MultiWriter(w, &sebagaiTeks{f}), func() { f.Close() }
}

// sebagaiTeks menulis byte mentah dalam bentuk yang bisa dibaca mata, supaya
// escape sequence terlihat sebagaimana adanya alih-alih dijalankan oleh
// terminal yang membuka berkasnya.
type sebagaiTeks struct{ w io.Writer }

func (t *sebagaiTeks) Write(p []byte) (int, error) {
	var b []byte
	for _, c := range p {
		switch {
		case c == 0x1b:
			b = append(b, []byte("<ESC>")...)
		case c == '\r':
			b = append(b, []byte("<CR>")...)
		case c == '\n':
			b = append(b, []byte("<LF>\n")...)
		case c == '\b':
			b = append(b, []byte("<BS>")...)
		case c == 0x07:
			b = append(b, []byte("<BEL>")...)
		case c < 0x20:
			b = append(b, []byte(fmt.Sprintf("<%02x>", c))...)
		default:
			b = append(b, c)
		}
	}
	if _, err := t.w.Write(b); err != nil {
		return 0, err
	}
	return len(p), nil
}
