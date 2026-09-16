package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// Options mengatur satu pemasangan.
type Options struct {
	// From adalah direktori sumber; kosong berarti memakai binary yang sedang
	// berjalan, yang hanya mungkin bila platformnya sama.
	From string
	// LocalSpecs adalah urutan direktori spec di mesin ini.
	LocalSpecs []string
	// Base adalah direktori tujuan di host, relatif terhadap rumah pengguna.
	Base string
	// Force memasang ulang meski versinya sudah sama.
	Force bool
	// DryRun hanya melaporkan rencana tanpa mengirim apa pun.
	DryRun bool
	// Version adalah versi anjuran lokal, dipakai membandingkan dengan yang terpasang.
	Version string
	// Out adalah tempat laporan ditulis.
	Out io.Writer
}

// Plan adalah apa yang akan dilakukan, disusun sebelum apa pun dikirim.
//
// Dipisahkan agar bisa ditampilkan lebih dulu: memasang berkas ke server orang
// lain adalah tindakan yang layak dilihat sebelum terjadi, bukan sesudah.
type Plan struct {
	Host      string
	Platform  Platform
	Source    Source
	Installed string
	BinPath   string
	SpecsPath string
	// UpToDate menandai host yang sudah memakai versi yang sama.
	UpToDate bool
}

// String merangkai rencana menjadi laporan yang bisa dibaca.
func (p Plan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "  host      : %s (%s)\n", p.Host, p.Platform)
	fmt.Fprintf(&b, "  sumber    : %s\n", p.Source.Origin)
	fmt.Fprintf(&b, "  binary    : %s\n", p.BinPath)
	if p.Source.Specs != "" {
		fmt.Fprintf(&b, "  spec      : %s\n", p.SpecsPath)
	} else {
		fmt.Fprintf(&b, "  spec      : tidak ada di sumber; completion akan kosong\n")
	}
	if p.Installed != "" {
		fmt.Fprintf(&b, "  terpasang : %s\n", p.Installed)
	}
	return b.String()
}

// Prepare menyusun rencana tanpa mengubah apa pun di host.
func Prepare(ctx context.Context, t Transport, opt Options) (Plan, func(), error) {
	noop := func() {}

	base := opt.Base
	if base == "" {
		base = RemoteBase
	}

	plat, err := Detect(ctx, t)
	if err != nil {
		return Plan{}, noop, err
	}

	binPath := path.Join(base, RemoteBin)
	plan := Plan{
		Host:      t.Target(),
		Platform:  plat,
		BinPath:   path.Join("~", binPath),
		SpecsPath: path.Join("~", base, RemoteSpecs),
	}

	plan.Installed = Installed(ctx, t, binPath)
	if plan.Installed != "" && opt.Version != "" &&
		strings.Contains(plan.Installed, opt.Version) && !opt.Force {
		plan.UpToDate = true
		return plan, noop, nil
	}

	src, cleanup, err := ResolveSource(opt.From, plat, opt.LocalSpecs)
	if err != nil {
		return Plan{}, noop, err
	}
	plan.Source = src
	return plan, cleanup, nil
}

// Install mengirim berkas dan memeriksa hasilnya.
func Install(ctx context.Context, t Transport, plan Plan, opt Options) error {
	base := opt.Base
	if base == "" {
		base = RemoteBase
	}
	out := opt.Out
	if out == nil {
		out = io.Discard
	}

	// Direktori dibuat lebih dulu dan terpisah, supaya kegagalan izin tulis
	// terlihat sebagai kegagalan mkdir — bukan sebagai arsip rusak.
	mkdir := fmt.Sprintf("mkdir -p %s %s",
		homePath(path.Join(base, "bin")),
		homePath(path.Join(base, "share/anjuran")))
	if _, err := t.Exec(ctx, mkdir); err != nil {
		return fmt.Errorf("menyiapkan direktori di %s: %w", t.Target(), err)
	}

	// Spec lama dibuang agar berkas yang sudah tidak ada di rilis baru tidak
	// tertinggal dan tetap ditawarkan.
	if plan.Source.Specs != "" {
		rm := "rm -rf " + homePath(path.Join(base, RemoteSpecs))
		if _, err := t.Exec(ctx, rm); err != nil {
			return fmt.Errorf("membersihkan spec lama: %w", err)
		}
	}

	pr, pw := io.Pipe()
	var sent int64
	go func() {
		n, err := Bundle(plan.Source.Binary, plan.Source.Specs, pw)
		sent = n
		pw.CloseWithError(err)
	}()

	extract := "tar xzf - -C " + homePath(base)
	if err := t.Send(ctx, extract, pr); err != nil {
		return fmt.Errorf("mengirim ke %s: %w", t.Target(), err)
	}
	fmt.Fprintf(out, "  terkirim  : %s\n", humanBytes(sent))

	// Berhasil menyalin belum berarti berhasil menjalankan. Home yang dipasang
	// noexec, atau berkas yang kehilangan bit eksekusinya, baru ketahuan di sini.
	version := Installed(ctx, t, path.Join(base, RemoteBin))
	if version == "" {
		return fmt.Errorf("berkas tersalin tetapi %s tidak bisa dijalankan di %s; "+
			"biasanya karena home dipasang noexec", plan.BinPath, t.Target())
	}
	fmt.Fprintf(out, "  terpasang : %s\n", version)
	return nil
}

// ShellHint adalah baris yang perlu ditambahkan pengguna di host.
func ShellHint(base string) string {
	if base == "" {
		base = RemoteBase
	}
	bin := path.Join("$HOME", base, "bin")
	return fmt.Sprintf(`Tambahkan ke berkas konfigurasi shell di host itu:

  export PATH="%s:$PATH"
  eval "$(anjuran init zsh)"      # atau bash, fish, powershell`, bin)
}

// humanBytes memformat ukuran agar mudah dibaca.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// Allowed memeriksa apakah host sudah disetujui lebih dulu lewat berkas.
//
// Berkas ini ada untuk pemakaian terskrip: persetujuan yang bisa ditinjau dan
// disimpan di kendali versi, bukan jawaban di layar yang tidak meninggalkan
// jejak. Pemakaian interaktif tidak memerlukannya.
func Allowed(host, file string) bool {
	b, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if ok, _ := path.Match(line, host); ok {
			return true
		}
		// Pola juga dicocokkan dengan bagian host saja, supaya "web-*"
		// berlaku untuk "deploy@web-01".
		if i := strings.LastIndex(host, "@"); i >= 0 {
			if ok, _ := path.Match(line, host[i+1:]); ok {
				return true
			}
		}
	}
	return false
}
