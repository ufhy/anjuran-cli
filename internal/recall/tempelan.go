package recall

import (
	"os"
	"path/filepath"
	"time"
)

// jendelaTempelan adalah lama sebuah tempelan dianggap masih berlangsung.
//
// Tempelan panjang tiba dalam satu tarikan, tetapi shell memprosesnya tombol
// demi tombol dan setiap pemicu menumbuhkan proses anjuran yang baru. Jendela
// ini harus cukup lebar untuk menutupi seluruh sisa tempelan, dan cukup
// sempit agar pengguna yang segera mengetik sesudah menempel tidak kehilangan
// completion-nya.
//
// 400 ms: satu baris perintah biasa selesai diproses jauh di bawah itu,
// sementara jeda antara menempel dan mulai mengetik hampir selalu lebih lama —
// mata perlu membaca dulu apa yang barusan mendarat.
const jendelaTempelan = 400 * time.Millisecond

// berkasTempelan adalah penanda bahwa sebuah tempelan sedang berlangsung.
//
// Ditulis sebagai berkas, bukan disimpan dalam memori, karena setiap penekanan
// tombol pemicu adalah PROSES BARU: yang mengetahui adanya tempelan sudah
// keluar sebelum pemicu berikutnya dijalankan.
func berkasTempelan() string {
	dir := CacheDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "tempelan")
}

// TandaiTempelan mencatat bahwa tempelan sedang berlangsung.
func TandaiTempelan() {
	p := berkasTempelan()
	if p == "" {
		return
	}
	// Isinya tidak dipakai; yang bermakna adalah waktu ubahnya. Menulis berkas
	// kosong sudah cukup, dan kegagalannya tidak perlu ditangani — kehilangan
	// penjagaan ini hanya membuat sebuah kotak muncul, bukan merusak apa pun.
	os.WriteFile(p, nil, 0o600)
}

// SedangMenempel menjawab apakah tempelan masih berlangsung.
func SedangMenempel() bool {
	p := berkasTempelan()
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}
	sisa := time.Since(fi.ModTime())
	// Waktu yang MUNDUR — jam sistem yang disetel ulang, atau berkas yang
	// tertinggal dari masa depan — tidak boleh mematikan completion selamanya.
	return sisa >= 0 && sisa < jendelaTempelan
}
