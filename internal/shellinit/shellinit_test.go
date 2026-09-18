package shellinit

import (
	"strings"
	"testing"
)

func TestSemuaShellPunyaSkrip(t *testing.T) {
	for _, sh := range Shells() {
		s, err := Script(sh)
		if err != nil {
			t.Errorf("Script(%q): %v", sh, err)
			continue
		}
		if len(s) < 200 {
			t.Errorf("skrip %s terlalu pendek (%d byte), kemungkinan gagal disematkan", sh, len(s))
		}
	}
}

func TestShellTakDikenalMenghasilkanError(t *testing.T) {
	if _, err := Script("tcsh"); err == nil {
		t.Error("mau error untuk shell yang tidak didukung")
	}
}

// Protokol widget hanya ada satu, jadi setiap skrip harus memanggilnya dengan
// cara yang sama. Uji ini menangkap skrip yang tertinggal saat protokolnya
// berubah.
func TestSetiapSkripMematuhiProtokol(t *testing.T) {
	for _, sh := range Shells() {
		s, _ := Script(sh)
		for _, want := range []string{"anjuran widget", "--line", "--cursor"} {
			if !strings.Contains(s, want) {
				t.Errorf("skrip %s tidak memuat %q", sh, want)
			}
		}
		// Ketiga status harus ditangani; "none" khususnya, karena itulah yang
		// mengembalikan tombol ke completion bawaan shell.
		if !strings.Contains(s, "none") {
			t.Errorf("skrip %s tidak menangani status none", sh)
		}
		if !strings.Contains(s, "ANJURAN_KEY") {
			t.Errorf("skrip %s tidak menghormati ANJURAN_KEY", sh)
		}
	}
}

// Setiap keluarga shell melaporkan posisi kursor dengan satuan berbeda, dan
// skrip yang salah meminta satuan akan menyisipkan di posisi keliru begitu
// baris memuat huruf non-ASCII.
func TestSatuanKursorPerShell(t *testing.T) {
	want := map[string]string{
		"bash":       "--cursor-unit byte",  // READLINE_POINT menghitung byte
		"powershell": "--cursor-unit utf16", // indeks string .NET
		"zsh":        "",                    // rune, yaitu nilai bawaan
		"fish":       "",
	}
	for sh, flag := range want {
		s, err := Script(sh)
		if err != nil {
			t.Fatalf("Script(%q): %v", sh, err)
		}
		if flag == "" {
			if strings.Contains(s, "--cursor-unit") {
				t.Errorf("skrip %s memakai satuan rune, seharusnya tidak menyebut --cursor-unit", sh)
			}
			continue
		}
		if !strings.Contains(s, flag) {
			t.Errorf("skrip %s harus memuat %q", sh, flag)
		}
	}
}

// pwsh adalah nama biner PowerShell 6 ke atas dan harus mengarah ke skrip yang
// sama, karena deteksi dari $SHELL akan menemukan nama itu.
func TestPwshAliasKePowershell(t *testing.T) {
	a, err := Script("pwsh")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Script("powershell")
	if a != b {
		t.Error("pwsh dan powershell harus memakai skrip yang sama")
	}
	for _, sh := range Shells() {
		if sh == "pwsh" {
			t.Error("pwsh tidak boleh muncul dua kali di daftar shell")
		}
	}
}

// Pemicunya bukan satu tombol, melainkan titik-titik di mana ada sesuatu yang
// layak ditawarkan — mengikuti cara IDE bekerja.
func TestZshPemicu(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{
		// Nyala secara bawaan, dimatikan dengan ANJURAN_AUTO=0. Sempat harus
		// dinyalakan sendiri, dan itu membuat mode utama alat ini tersembunyi
		// di balik variabel yang harus diketahui namanya lebih dulu.
		`${ANJURAN_AUTO:-1} != (0|no|off|false)`,

		`bindkey " " _anjuran_spasi`, // spasi
		`bindkey "/" _anjuran_garismiring`,
		`bindkey "=" _anjuran_samadengan`,
		"viins", // mode vi memakai keymap terpisah
	} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
}

// Widget yang sudah terpasang pada tombol pemicu dipanggil lebih dulu, supaya
// perilakunya tetap utuh: oh-my-zsh memetakan spasi ke magic-space.
func TestZshMembungkusWidgetYangAda(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{"bindkey ", "magic-space", "_anjuran_asli"} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
}

// Tombol yang bukan urusan dropdown dikembalikan ke antrean masukan zsh.
// Tanpa itu, sesi yang memegang masukan akan menelan tombol seperti Ctrl-A.
func TestZshMengembalikanTombolSisa(t *testing.T) {
	s, _ := Script("zsh")
	for _, want := range []string{"zle -U", "_anjuran_kembalikan"} {
		if !strings.Contains(s, want) {
			t.Errorf("skrip zsh tidak memuat %q", want)
		}
	}
}

// Mengetik huruf TIDAK membuka kotak.
//
// Sempat dibuat begitu, meniru editor yang memunculkan daftar sambil nama
// diketik, dan di shell itu terasa mengganggu: kotak berkedip pada hampir
// setiap kata. Yang membuka kotak hanyalah karakter yang MENGAKHIRI kata.
func TestZshTidakMembungkusSelfInsert(t *testing.T) {
	s, _ := Script("zsh")
	for _, larang := range []string{"zle -N self-insert", "zle -N backward-delete-char"} {
		// Bayangan riwayat memasang keduanya untuk keperluannya sendiri;
		// yang diperiksa di sini adalah blok pemicu.
		pemicu := s
		if i := strings.Index(s, "Saran dari riwayat"); i > 0 {
			pemicu = s[:i]
		}
		if strings.Contains(pemicu, larang) {
			t.Errorf("blok pemicu tidak boleh memuat %q", larang)
		}
	}
}

// Pemicu otomatis harus sama di setiap shell yang punya integrasi.
//
// Pemicu yang berbeda antar shell membuat alat yang sama terasa seperti dua
// alat berbeda begitu seseorang berpindah mesin — dan itu paling terasa lewat
// SSH, tempat shell di seberang sering bukan shell yang dipakai sehari-hari.
var pengikatan = map[string][]string{
	"zsh":        {`bindkey " " _anjuran_spasi`, `bindkey "/" _anjuran_garismiring`, `bindkey "=" _anjuran_samadengan`},
	"bash":       {`bind -x '" ": _anjuran_spasi'`, `bind -x '"/": _anjuran_garismiring'`, `bind -x '"=": _anjuran_samadengan'`},
	"fish":       {`bind ' ' _anjuran_spasi`, `bind / _anjuran_garismiring`, `bind = _anjuran_samadengan`},
	// PowerShell memicu pada "\" juga: path Windows tidak memakai garis
	// miring, sehingga tanpa itu pemicu path di shell ini tidak pernah
	// ditekan siapa pun.
	"powershell": {`@(' ', '/', '\', '=')`, `::Insert('$karakter')`},
}

func TestPemicuOtomatisSamaDiSetiapShell(t *testing.T) {
	for _, sh := range []string{"zsh", "bash", "fish", "powershell"} {
		s, _ := Script(sh)
		if !strings.Contains(s, "ANJURAN_AUTO") {
			t.Errorf("skrip %s harus menghormati ANJURAN_AUTO", sh)
		}
		// Yang diperiksa baris PENGIKATANNYA, bukan sekadar karakternya:
		// spasi dan "=" muncul di mana-mana dalam skrip mana pun, sehingga
		// mencarinya begitu saja adalah uji yang tidak pernah bisa gagal.
		for _, ikat := range pengikatan[sh] {
			if !strings.Contains(s, ikat) {
				t.Errorf("skrip %s tidak memasang pemicu: %q", sh, ikat)
			}
		}
	}
}

// Setiap pemicu otomatis harus menyisipkan karakternya sendiri.
//
// Di keempat shell, mengikat sebuah karakter berarti mengambil alih tombolnya
// sepenuhnya. Lupa menyisipkannya kembali membuat karakter yang diketik
// pengguna hilang — gejala yang tampak seperti keyboard rusak, bukan seperti
// completion yang salah.
func TestPemicuMenyisipkanKarakternya(t *testing.T) {
	sisip := map[string]string{
		"zsh":        ".self-insert",
		"bash":       "_anjuran_sisip",
		"fish":       "commandline -i",
		"powershell": "::Insert(",
	}
	for sh, tanda := range sisip {
		s, _ := Script(sh)
		if !strings.Contains(s, tanda) {
			t.Errorf("skrip %s tidak menyisipkan karakter pemicunya (%q)", sh, tanda)
		}
	}
}

// Spasi belum tentu terpasang ke self-insert: oh-my-zsh memetakannya ke
// magic-space. Membungkus self-insert saja berarti fitur ini mati diam-diam
// di konfigurasi yang justru paling banyak dipakai.

// Turun ke dalam folder dan pemekaran alias harus ada di setiap shell.
//
// Keduanya tidak punya penghalang teknis di shell mana pun: yang pertama
// hanyalah widget yang memanggil dirinya sendiri dengan --select none, yang
// kedua hanyalah pertanyaan "apa arti kata ini" yang setiap shell bisa jawab.
// Mendaratkannya di zsh saja adalah pekerjaan yang berhenti separuh jalan,
// bukan batas yang dipaksakan shell.
func TestTurunFolderDanAliasDiSetiapShell(t *testing.T) {
	for _, sh := range []string{"zsh", "bash", "fish", "powershell"} {
		s, _ := Script(sh)
		if !strings.Contains(s, "--select") && !strings.Contains(s, "-Select") {
			t.Errorf("skrip %s tidak pernah mengirim --select; turun folder tidak mungkin", sh)
		}
		if !strings.Contains(s, "--alias") && !strings.Contains(s, "-Alias") {
			t.Errorf("skrip %s tidak pernah mengirim --alias", sh)
		}
		// Rekursinya harus dibuka TANPA sorotan, kalau tidak penelusuran
		// tidak punya cara berhenti dan terus turun sampai dasar.
		if !strings.Contains(s, "none") {
			t.Errorf("skrip %s membuka isi folder dengan sorotan; tidak ada cara berhenti", sh)
		}
	}
}

// ANJURAN_GHOST harus punya arti di setiap shell yang bisa menampilkannya.
//
// zsh menulis implementasinya sendiri karena zsh tidak punya padanannya.
// fish dan PSReadLine SUDAH punya, dan miliknya lebih baik — jadi yang benar
// memakai milik mereka, bukan menirunya dan menumpuk dua teks abu-abu.
func TestGhostTextMemakaiFiturBawaanShell(t *testing.T) {
	tanda := map[string]string{
		"zsh":        "POSTDISPLAY",
		"fish":       "fish_autosuggestion_enabled",
		"powershell": "PredictionSource",
	}
	for sh, t2 := range tanda {
		s, _ := Script(sh)
		if !strings.Contains(s, "ANJURAN_GHOST") {
			t.Errorf("skrip %s tidak menghormati ANJURAN_GHOST", sh)
		}
		if !strings.Contains(s, t2) {
			t.Errorf("skrip %s tidak memakai %s", sh, t2)
		}
	}

	// bash memang tidak bisa: readline tidak punya kait per-ketikan maupun
	// tempat menggambar teks di luar buffer. Yang diwajibkan di sini bukan
	// fiturnya, melainkan KETERANGANNYA — supaya ketiadaannya tidak lagi
	// ditemukan orang dengan cara mencobanya.
	b, _ := Script("bash")
	if !strings.Contains(b, "ANJURAN_GHOST") {
		t.Error("skrip bash tidak menyebut ANJURAN_GHOST sama sekali")
	}
}

// Kurung kurawal skrip PowerShell harus seimbang.
//
// Sebuah kurawal berlebih membuat `anjuran init powershell` menghasilkan
// skrip yang GAGAL DIURAI — integrasinya mati seluruhnya, bukan cacat
// sebagian. Dan kegagalannya diam: `Invoke-Expression` mengeluh ke stderr
// lalu prompt tetap muncul seperti biasa, jadi tidak ada yang tampak rusak
// sampai seseorang menekan Tab dan tidak terjadi apa-apa.
//
// Pemeriksaan ini kasar dan tidak menggantikan parser sungguhan, tetapi
// menangkap justru kesalahan yang paling mudah dibuat saat menyunting skrip
// ini dari luar PowerShell.
func TestPowerShellKurawalSeimbang(t *testing.T) {
	s, _ := Script("powershell")

	dalamKomentar := false
	imbang := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			dalamKomentar = false
		case '#':
			dalamKomentar = true
		case '{':
			if !dalamKomentar {
				imbang++
			}
		case '}':
			if !dalamKomentar {
				imbang--
			}
		}
		if imbang < 0 {
			t.Fatalf("kurawal penutup berlebih pada offset %d", i)
		}
	}
	if imbang != 0 {
		t.Errorf("kurawal tidak seimbang: selisih %d", imbang)
	}
}

// Baris perintah harus tetap terlihat saat kotak dibuka.
//
// bash MENGHAPUS baris yang terlihat sebelum menjalankan perintah `bind -x`
// dan baru menggambarnya ulang sesudah perintah itu selesai; fish tidak
// menggambar karakter yang disisipkan sampai binding-nya selesai. Pada
// keduanya, kotak digambar SELAMA fungsi berjalan — sehingga tanpa penanganan
// khusus, mengetik "cd " menampilkan daftar direktori tanpa satu pun jejak
// perintah yang sedang ditulis.
//
// Buffer-nya sendiri selalu benar, dan itu yang membuat cacat seperti ini
// bertahan lama: perintahnya tetap berjalan sebagaimana mestinya.
func TestBarisTetapTergambarSaatKotakDibuka(t *testing.T) {
	b, _ := Script("bash")
	// Prompt ikut digambar ulang, bukan hanya isinya: yang dihapus bash
	// adalah seluruh baris, prompt dan semuanya.
	for _, tanda := range []string{"_anjuran_gambar_baris", "${PS1@P}", "READLINE_LINE"} {
		if !strings.Contains(b, tanda) {
			t.Errorf("skrip bash tidak menggambar ulang barisnya (%q tidak ada)", tanda)
		}
	}

	f, _ := Script("fish")
	if !strings.Contains(f, "/dev/tty") {
		t.Error("skrip fish tidak menggemakan karakter pemicunya ke terminal")
	}

	// zsh memakai `zle -R`, yang menggambar SAAT ITU JUGA. `zle redisplay`
	// hanya menandai baris perlu digambar ulang, dan penggambarannya tetap
	// menunggu widget selesai — persis kegagalan yang sama.
	z, _ := Script("zsh")
	if !strings.Contains(z, "zle -R") {
		t.Error("skrip zsh tidak memakai zle -R")
	}
}

// Sesi yang selesai harus meninggalkan baris yang BERSIH.
//
// anjuran mengembalikan kursor ke posisi yang disimpan saat kotak dibuka —
// akhir baris perintah. bash lalu menggambar ulang barisnya sendiri mulai dari
// posisi itu, sehingga gambarnya menempel di belakang gambar pertama:
//
//	bash-5.3# anjuran bash-5.3# anjuran
//
// Menghapus baris sebelum kembali membuat gambar ulang bash mendarat di kolom
// nol, menggantikan yang lama alih-alih menyambungnya.
func TestBashMembersihkanBarisSebelumKembali(t *testing.T) {
	s, _ := Script("bash")
	if !strings.Contains(s, `_anjuran_widget_isi`) {
		t.Error("sesi bash tidak dibungkus, jadi tidak ada tempat membersihkan barisnya")
	}
	if !strings.Contains(s, `printf '\r\033[K'`) {
		t.Error("baris tidak dibersihkan sesudah sesi selesai")
	}
}
