package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/uf-cli/uf/internal/engine"
	"github.com/uf-cli/uf/internal/spec"
	"github.com/uf-cli/uf/internal/ui"
	"golang.org/x/term"
)

// runRender menggambar dropdown lalu langsung selesai, tanpa mengambil alih
// masukan sama sekali.
//
// Mode ini yang membuat dropdown bisa muncul sambil mengetik. Bedanya dengan
// widget: widget menguasai terminal dan membaca tombol sendiri, sedangkan di
// sini kendali dikembalikan ke shell seketika — shell tetap yang membaca
// ketikan, uf hanya menggambar.
//
// Karena itu pula mode ini TIDAK PERNAH membaca dari terminal. Membaca berarti
// berebut dengan shell yang sedang menunggu ketikan berikutnya, dan huruf yang
// sedang diketik pengguna bisa tertelan.
func runRender(args []string) int {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	line := fs.String("line", "", "isi buffer shell")
	cursor := fs.Int("cursor", -1, "posisi kursor")
	unit := fs.String("cursor-unit", "rune", "satuan posisi kursor: rune, byte, atau utf16")
	prev := fs.Int("prev-lines", 0, "jumlah baris yang sudah digambar sebelumnya")
	clear := fs.Bool("clear", false, "hapus dropdown lalu selesai")
	alias := fs.String("alias", "", "pemekaran alias untuk kata pertama")
	specsDir := fs.String("specs", "", "direktori spec")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Ditulis ke /dev/tty, bukan stdout: stdout dipakai membawa jumlah baris
	// kembali ke shell dan harus tetap bersih.
	tty, err := os.OpenFile(ttyDevice, os.O_WRONLY, 0)
	if err != nil {
		fmt.Println(0)
		return 0
	}
	defer tty.Close()

	w, h := 80, 24
	if tw, th, err := term.GetSize(int(tty.Fd())); err == nil && tw > 0 && th > 0 {
		w, h = tw, th
	}
	rend := ui.NewRenderer(tty, w, h, simpleMode())
	rend.Adopt(*prev)

	if *clear {
		rend.Clear()
		fmt.Println(0)
		return 0
	}

	dirs, err := resolveSpecsDirs(*specsDir)
	if err != nil {
		fmt.Println(*prev)
		return 0
	}

	eng := engine.New(spec.NewRegistryDirs(dirs...))
	byteCursor := toByteCursor(*line, *cursor, *unit)
	ax := newAliasExpansion(*line, byteCursor, *alias)
	st := ui.State{Line: ax.Line(*line), Cursor: ax.Cursor(byteCursor)}

	pre, err := ui.Prepare(eng, st, newDynamic())
	if err != nil {
		fmt.Println(*prev)
		return 0
	}

	n := rend.Show(pre.Candidates(), pre.Prefix())
	fmt.Println(n)
	return 0
}
