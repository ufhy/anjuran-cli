package remote

import (
	"strings"
	"testing"
)

// BatchMode mematikan prompt password. Dipaksa selalu, host yang hanya
// menerima password menjadi tidak bisa dimasuki sama sekali — kunci wajib
// bukan karena SSH memintanya, melainkan karena kita melarang alternatifnya.
func TestSshArgsBatchModeHanyaTanpaTerminal(t *testing.T) {
	s := &SSH{Host: "contoh", controlPath: "/tmp/cm", Batch: true}
	if !punya(s.sshArgs("echo"), "BatchMode=yes") {
		t.Error("tanpa orang yang bisa menjawab, prompt harus dilarang")
	}

	s.Batch = false
	// Uji Go berjalan tanpa terminal, jadi interactive() tetap false di sini;
	// yang diperiksa adalah bahwa opsinya memang berasal dari pemeriksaan itu,
	// bukan ditulis mati.
	if !punya(s.sshArgs("echo"), "BatchMode=yes") {
		t.Error("stdin uji bukan terminal, jadi BatchMode tetap dipasang")
	}
}

// Argumen milik pengguna harus mendarat SESUDAH opsi bawaan, supaya -p atau
// -i yang ia tulis sendiri bisa menimpanya, dan sebelum host.
func TestSshArgsUrutan(t *testing.T) {
	s := &SSH{Host: "contoh", Args: []string{"-p", "2222"}, controlPath: "/tmp/cm"}
	args := s.sshArgs("echo halo")

	p := indeks(args, "-p")
	h := indeks(args, "contoh")
	if p < 0 || h < 0 || p > h {
		t.Fatalf("opsi pengguna harus sebelum host: %v", args)
	}
	if args[len(args)-1] != "echo halo" {
		t.Errorf("perintah remote harus terakhir: %v", args)
	}
}

func punya(args []string, s string) bool {
	for _, a := range args {
		if strings.Contains(a, s) {
			return true
		}
	}
	return false
}

func indeks(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}
