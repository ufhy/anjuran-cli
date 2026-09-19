package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ufhy/anjuran-cli/internal/spec"
)

// Pemeriksaan untuk spec tulisan tangan di extra/.
//
// extra/ adalah satu-satunya tempat yang boleh menerima sumbangan orang luar,
// dan satu-satunya tempat yang generatornya DIPERCAYA — di sanalah larangan
// interpreter dilonggarkan, karena isinya ditinjau satu per satu. Dua sifat itu
// berbahaya bila bertemu tanpa pemeriksaan: menerima spec berarti menerima
// perintah yang akan dijalankan di mesin orang lain, sebagai efek samping
// mengetik.
//
// Berkas ini adalah bagian "tidak sekadar ditinjau manusia" dari penjagaan itu.
// Peninjau bisa lelah dan bisa terburu-buru; pemeriksaan di bawah tidak.

const dirExtra = "../../extra"

// dapatExtra membaca seluruh spec di extra/.
func dapatExtra(t *testing.T) map[string]*spec.Subcommand {
	t.Helper()

	entri, err := os.ReadDir(dirExtra)
	if err != nil {
		t.Skipf("extra/ tidak terbaca: %v", err)
	}

	out := map[string]*spec.Subcommand{}
	for _, en := range entri {
		if en.IsDir() || filepath.Ext(en.Name()) != ".json" {
			continue
		}
		p := filepath.Join(dirExtra, en.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", en.Name(), err)
		}
		var sc spec.Subcommand
		if err := json.Unmarshal(b, &sc); err != nil {
			t.Errorf("%s: tidak bisa diurai: %v", en.Name(), err)
			continue
		}
		out[en.Name()] = &sc
	}
	if len(out) == 0 {
		t.Skip("tidak ada spec di extra/")
	}
	return out
}

// Nama berkas harus sama dengan nama perintahnya.
//
// Spec di extra/ DIGABUNG di atas korpus berdasarkan nama berkasnya. Berkas
// yang namanya meleset tidak akan pernah terpakai, dan kegagalannya diam
// total: penyumbangnya melihat specnya masuk, penggunanya tidak melihat
// perubahan apa pun, dan tidak ada yang mencetak sepatah kata.
func TestExtraNamaBerkasCocokDenganPerintah(t *testing.T) {
	for nama, sc := range dapatExtra(t) {
		perintah := strings.TrimSuffix(nama, ".json")
		if len(sc.Name) == 0 {
			t.Errorf("%s: tidak menyebut nama perintah", nama)
			continue
		}
		cocok := false
		for _, n := range sc.Name {
			if n == perintah {
				cocok = true
				break
			}
		}
		if !cocok {
			t.Errorf("%s: berisi spec untuk %v, jadi ia tidak akan pernah "+
				"digabung ke %q", nama, []string(sc.Name), perintah)
		}
	}
}

// Generator hanya boleh menjalankan PERINTAHNYA SENDIRI.
//
// Ini aturan yang sama yang berlaku saat dijalankan — anjuran tidak memanggil
// program yang tidak sedang kamu jalankan sendiri — tetapi diperiksa lebih
// awal, saat spec masuk, bukan saat seseorang mengetik. Spec yang melanggarnya
// akan ditolak diam-diam oleh kebijakan di mesin pengguna, dan "sudah aman
// karena nanti ditolak" bukan alasan untuk menerimanya: yang masuk ke repo
// adalah yang dibaca orang sebagai contoh.
func TestExtraGeneratorHanyaMenjalankanPerintahnyaSendiri(t *testing.T) {
	for nama, sc := range dapatExtra(t) {
		perintah := strings.TrimSuffix(nama, ".json")
		for _, g := range kumpulkanGenerator(sc) {
			if len(g.Script) == 0 {
				continue
			}
			biner := filepath.Base(g.Script[0])
			if biner != perintah {
				t.Errorf("%s: generator menjalankan %q, bukan %q — "+
					"kebijakan akan menolaknya di mesin pengguna",
					nama, biner, perintah)
			}
		}
	}
}

// Generator tidak boleh menerima KODE sebagai argumen.
//
// Spec di extra/ ditandai trusted, dan penandaan itu melonggarkan larangan
// interpreter — sebuah kelonggaran yang bertumpu sepenuhnya pada tinjauan
// manusia. Bentuk yang paling berbahaya punya ciri yang bisa dikenali mesin:
// flag yang isinya dieksekusi. "php artisan list" adalah data; "php -r <apa
// pun>" adalah kode, dan yang menulis isinya belum tentu penyumbang specnya.
func TestExtraGeneratorTidakMenerimaKode(t *testing.T) {
	// Flag yang argumennya dijalankan sebagai kode, di lintas bahasa.
	flagKode := map[string]bool{
		"-c": true, "-e": true, "-r": true, "--eval": true,
		"--command": true, "--exec": true, "-exec": true,
	}

	for nama, sc := range dapatExtra(t) {
		for _, g := range kumpulkanGenerator(sc) {
			for _, arg := range g.Script {
				if flagKode[arg] {
					t.Errorf("%s: generator memakai %q; argumennya dijalankan "+
						"sebagai kode, bukan dibaca sebagai data", nama, arg)
				}
				// Metakarakter shell di dalam argumen berarti seseorang
				// mengharapkan sebuah shell menafsirkannya. Tidak ada shell di
				// jalur ini — argv dijalankan langsung — jadi kehadirannya
				// menandakan spec yang ditulis dengan anggapan keliru, dan
				// anggapan keliru tentang eksekusi adalah tempat lubang lahir.
				if strings.ContainsAny(arg, "|;&$`") {
					t.Errorf("%s: argumen generator %q memuat metakarakter "+
						"shell; argv dijalankan langsung, tanpa shell", nama, arg)
				}
			}
		}
	}
}

// Setiap spec harus MENJAWAB, bukan sekadar terurai.
//
// Spec yang terurai bersih tetapi tidak menawarkan apa pun adalah kegagalan
// yang paling mahal untuk ditemukan: tidak ada pesan galat, tidak ada tanda,
// hanya Tab yang diam. Karena itu setiap berkas menyebut sendiri baris yang
// harus dijawabnya, di dalam berkas yang sama supaya keduanya tidak pernah
// berpisah.
func TestExtraMenjawabBarisContohnya(t *testing.T) {
	e := New(spec.NewRegistry(dirExtra))

	for nama, sc := range dapatExtra(t) {
		if len(sc.Contoh) == 0 {
			t.Errorf("%s: tidak menyebut satu pun baris contoh "+
				"(anjuranContoh); tanpa itu tidak ada yang membuktikan "+
				"spec ini menjawab", nama)
			continue
		}
		for _, baris := range sc.Contoh {
			res, err := e.Complete(baris, len(baris))
			if err != nil {
				t.Errorf("%s: %q gagal: %v", nama, baris, err)
				continue
			}
			// Template dihitung sebagai jawaban: "cd " menjawab dengan isi
			// direktori, dan isinya baru ada saat dijalankan.
			if len(res.Candidates) == 0 && len(res.Templates) == 0 &&
				len(res.Generators) == 0 {
				t.Errorf("%s: %q tidak menjawab apa pun", nama, baris)
			}
		}
	}
}

// kumpulkanGenerator menelusuri seluruh generator di dalam sebuah spec.
func kumpulkanGenerator(sc *spec.Subcommand) []spec.Generator {
	var out []spec.Generator
	var arg func(a *spec.Arg)
	arg = func(a *spec.Arg) {
		out = append(out, a.Generators...)
	}
	var turun func(s *spec.Subcommand)
	turun = func(s *spec.Subcommand) {
		for i := range s.Args {
			arg(&s.Args[i])
		}
		for i := range s.Options {
			for j := range s.Options[i].Args {
				arg(&s.Options[i].Args[j])
			}
		}
		for i := range s.Subcommands {
			turun(&s.Subcommands[i])
		}
	}
	turun(sc)
	return out
}
