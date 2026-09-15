package generator

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Batas yang dipasang pada setiap eksekusi. Angkanya dipilih untuk melindungi
// pengalaman mengetik, bukan untuk kebenaran: generator yang lambat atau
// bocor tidak boleh menahan tombol Tab.
const (
	// DefaultTimeout adalah batas waktu satu generator.
	DefaultTimeout = 1200 * time.Millisecond
	// maxOutput membatasi keluaran yang dibaca. Perintah yang keliru bisa
	// mencetak tanpa henti.
	maxOutput = 1 << 20 // 1 MiB
	// maxCandidates membatasi jumlah baris yang diolah; dropdown tidak
	// pernah menampilkan lebih dari belasan.
	maxCandidates = 2000
	// failureTTL adalah masa berlaku cache untuk generator yang gagal.
	failureTTL = 3 * time.Second
)

// Spec adalah generator yang akan dijalankan, bentuk minimal yang dibutuhkan
// paket ini. Dipisah dari tipe di paket spec agar generator tidak bergantung
// pada skema berkas.
type Spec struct {
	Script   []string
	SplitOn  string
	Trim     bool
	CacheTTL time.Duration
	// Trusted menandai generator dari spec buatan tangan.
	Trusted bool
}

// Runner menjalankan generator sesuai kebijakan.
type Runner struct {
	Policy  Policy
	Timeout time.Duration
	Cache   *Cache
	// Dir adalah direktori kerja; kosong berarti direktori proses.
	Dir string
	// Denied menampung alasan penolakan, untuk ditampilkan bila diminta.
	Denied []string
}

// Run menjalankan satu generator dan mengembalikan kandidatnya.
//
// Kegagalan bukan error bagi pemanggil: generator yang ditolak, habis waktu,
// atau programnya tidak terpasang hanya menghasilkan daftar kosong. Tab yang
// tidak menawarkan apa-apa jauh lebih baik daripada Tab yang menampilkan pesan
// kesalahan di tengah baris perintah.
func (r *Runner) Run(ctx context.Context, g Spec) []Candidate {
	if len(g.Script) == 0 {
		return nil
	}

	if d := r.Policy.Check(g.Script, g.Trusted); !d.Allowed {
		r.Denied = append(r.Denied, strings.Join(g.Script, " ")+": "+d.Reason)
		return nil
	}

	if r.Cache != nil {
		if out, ok := r.Cache.Get(g.Script, r.Dir); ok {
			return parse(out, g)
		}
	}

	out, err := r.exec(ctx, g.Script)
	if err != nil {
		// Kegagalan ikut di-cache, dengan masa berlaku pendek.
		//
		// Tanpa ini, mengetik di direktori yang bukan repo git akan menjalankan
		// `git branch` yang gagal berulang-ulang — dan generator yang gagal
		// justru yang paling mahal, karena biayanya dibayar penuh setiap kali.
		// Masa berlakunya sengaja pendek: keadaan yang membuatnya gagal bisa
		// berubah kapan saja, misalnya setelah `git init`.
		if r.Cache != nil {
			r.Cache.Put(g.Script, r.Dir, "", failureTTL)
		}
		return nil
	}

	if r.Cache != nil && g.CacheTTL > 0 {
		r.Cache.Put(g.Script, r.Dir, out, g.CacheTTL)
	}
	return parse(out, g)
}

// exec menjalankan argv secara langsung, TANPA shell. Tidak ada string yang
// pernah diurai sebagai perintah, sehingga isi berkas spec tidak bisa menjadi
// jalur injeksi.
func (r *Runner) exec(ctx context.Context, argv []string) (string, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = r.Dir
	cmd.Stdin = nil // generator tidak pernah membaca masukan
	cmd.Stderr = io.Discard

	var buf bytes.Buffer
	cmd.Stdout = &buf

	// Proses anak ditaruh di grup sendiri agar seluruh keturunannya ikut mati
	// saat waktu habis; tanpa ini, cucu proses bisa tertinggal hidup.
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return "", err
		}
	case <-ctx.Done():
		killGroup(cmd)
		<-done
		return "", ctx.Err()
	}

	s := buf.String()
	if len(s) > maxOutput {
		s = s[:maxOutput]
	}
	return s, nil
}

// parse memecah keluaran menjadi kandidat yang sudah dinormalkan.
func parse(out string, g Spec) []Candidate {
	sep := g.SplitOn
	if sep == "" {
		sep = "\n"
	}

	seen := make(map[string]bool)
	var cands []Candidate
	for _, line := range strings.Split(out, sep) {
		c, ok := normalize(line)
		if !ok || seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		cands = append(cands, c)
		if len(cands) >= maxCandidates {
			break
		}
	}
	return cands
}

// RunAll menjalankan beberapa generator secara berurutan di bawah satu
// tenggat bersama, sehingga total waktunya tetap terbatas berapa pun jumlah
// generator yang dimiliki sebuah argumen.
func (r *Runner) RunAll(specs []Spec) []Candidate {
	if len(specs) == 0 {
		return nil
	}

	budget := r.Timeout
	if budget <= 0 {
		budget = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	seen := make(map[string]bool)
	var out []Candidate
	for _, g := range specs {
		for _, c := range r.Run(ctx, g) {
			if !seen[c.Name] {
				seen[c.Name] = true
				out = append(out, c)
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return out
}

// CurrentDir mengembalikan direktori kerja, atau kosong bila tidak terbaca.
func CurrentDir() string {
	d, err := os.Getwd()
	if err != nil {
		return ""
	}
	return d
}
