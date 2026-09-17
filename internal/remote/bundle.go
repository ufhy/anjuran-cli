package remote

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Layout adalah letak berkas di host tujuan, relatif terhadap direktori dasar.
//
// Mengikuti tata letak XDG supaya penemuan spec berjalan tanpa variabel
// lingkungan: anjuran mencari ../share/anjuran/specs relatif terhadap binary-nya, dan
// bin/anjuran dengan share/anjuran/specs memenuhi itu persis.
const (
	RemoteBase  = ".local"
	RemoteBin   = "bin/anjuran"
	RemoteSpecs = "share/anjuran/specs"
	// RemoteExtra menampung tambalan buatan tangan. Ia ikut dikirim karena di
	// situlah pengetahuan yang TIDAK ada di korpus Fig: cd, ssh, docker,
	// kubectl, serta daftar skrip proyek. Tanpa itu completion di host remote
	// diam-diam lebih buruk daripada di mesin sendiri — dan yang paling
	// terasa justru di host remote, tempat perintahnya paling tidak dihafal.
	RemoteExtra = "share/anjuran/extra"
)

// Bundle menulis arsip tar.gz berisi binary dan spec.
//
// Arsip dialirkan langsung ke stdin perintah remote, tanpa pernah menjadi
// berkas di mesin lokal: satu proses, satu koneksi, tanpa sisa.
func Bundle(binPath, specsDir, extraDir string, w io.Writer) (int64, error) {
	counter := &countingWriter{w: w}
	zw := gzip.NewWriter(counter)
	tw := tar.NewWriter(zw)

	if err := addFile(tw, binPath, RemoteBin, 0o755); err != nil {
		return counter.n, err
	}
	if specsDir != "" {
		if err := addTree(tw, specsDir, RemoteSpecs); err != nil {
			return counter.n, err
		}
	}
	if extraDir != "" {
		if err := addTree(tw, extraDir, RemoteExtra); err != nil {
			return counter.n, err
		}
	}

	if err := tw.Close(); err != nil {
		return counter.n, err
	}
	if err := zw.Close(); err != nil {
		return counter.n, err
	}
	return counter.n, nil
}

func addFile(tw *tar.Writer, src, dst string, mode int64) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name:     dst,
		Mode:     mode,
		Size:     fi.Size(),
		Typeflag: tar.TypeReg,
		ModTime:  fi.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

// addTree menambahkan seluruh isi direktori secara rekursif.
func addTree(tw *tar.Writer, root, prefix string) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := path.Join(prefix, filepath.ToSlash(rel))

		if d.IsDir() {
			return tw.WriteHeader(&tar.Header{
				Name: name + "/", Mode: 0o755, Typeflag: tar.TypeDir,
			})
		}
		// Hanya berkas biasa yang dibawa; symlink dan berkas perangkat tidak
		// punya arti di direktori spec dan hanya menambah permukaan risiko.
		if !d.Type().IsRegular() {
			return nil
		}
		return addFile(tw, p, name, 0o644)
	})
}

// countingWriter menghitung byte yang benar-benar terkirim.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// Source menyebutkan asal binary dan spec yang akan dikirim.
type Source struct {
	Binary string
	Specs  string
	// Extra adalah tambalan buatan tangan; boleh kosong.
	Extra string
	// Origin menjelaskan asalnya untuk ditampilkan.
	Origin string
}

// ResolveSource menentukan berkas apa yang dikirim untuk sebuah platform.
//
// from boleh kosong, sebuah direktori berisi anjuran dan specs, atau direktori
// hasil rilis yang memuat arsip goreleaser.
func ResolveSource(from string, p Platform, localSpecs, localExtra []string) (Source, func(), error) {
	noop := func() {}

	if from != "" {
		return fromDir(from, p, noop)
	}

	// Platform host berbeda dari mesin ini, jadi binary yang sedang berjalan
	// tidak bisa dipakai. Itu keadaan yang lumrah — memasang dari laptop macOS
	// ke server Linux adalah kasus yang paling sering — jadi ia tidak pantas
	// langsung menjadi kegagalan yang menyuruh orang membangun sendiri.
	// Dicari dulu di tempat yang wajar, lalu dibangun kalau memang bisa.
	if p.OS != runtime.GOOS || p.Arch != runtime.GOARCH {
		if dir := cariBinaryLintas(p); dir != "" {
			return fromDir(dir, p, noop)
		}
		if src, bersihkan, err := bangunLintas(p, localSpecs, localExtra); err == nil {
			return src, bersihkan, nil
		} else if !errors.Is(err, errTakBisaBangun) {
			return Source{}, noop, err
		}
		return Source{}, noop, fmt.Errorf(
			"host adalah %s sedangkan mesin ini %s/%s; bangun binary-nya lalu sebutkan dengan --from (misalnya `make cross` atau `make snapshot`)",
			p, runtime.GOOS, runtime.GOARCH)
	}

	exe, err := os.Executable()
	if err != nil {
		return Source{}, noop, err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}

	specs := ""
	for _, d := range localSpecs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			specs = d
			break
		}
	}
	if specs == "" {
		return Source{}, noop, fmt.Errorf("direktori spec tidak ditemukan di mesin ini; jalankan `make specs`")
	}
	return Source{
		Binary: exe,
		Specs:  specs,
		Extra:  pertamaYangAda(localExtra),
		Origin: "binary anjuran yang sedang berjalan",
	}, noop, nil
}

// fromDir mencari sumber di dalam sebuah direktori.
func fromDir(dir string, p Platform, noop func()) (Source, func(), error) {
	// Bentuk pertama: direktori yang sudah berisi anjuran dan specs.
	//
	// Nama berakhiran platform ikut dikenali, karena itulah yang dihasilkan
	// `make cross`: bin/anjuran-linux-arm64. Tanpa ini pengguna harus menyalin
	// dan mengganti namanya sendiri hanya untuk memakai keluaran perintah
	// build milik proyek ini sendiri.
	ext := ""
	if p.OS == "windows" {
		ext = ".exe"
	}
	bin := ""
	for _, nama := range []string{
		"anjuran" + ext,
		fmt.Sprintf("anjuran-%s-%s%s", p.OS, p.Arch, ext),
	} {
		kandidat := filepath.Join(dir, nama)
		if fi, err := os.Stat(kandidat); err == nil && !fi.IsDir() {
			bin = kandidat
			break
		}
	}
	if bin != "" {
		return Source{
			Binary: bin,
			Specs:  subdirJikaAda(dir, "specs"),
			Extra:  subdirJikaAda(dir, "extra"),
			Origin: dir,
		}, noop, nil
	}

	// Bentuk kedua: direktori hasil rilis berisi arsip per platform.
	if arc := findArchive(dir, p); arc != "" {
		return extractArchive(arc, p)
	}

	return Source{}, noop, fmt.Errorf(
		"tidak menemukan anjuran untuk %s di %s; isinya harus berupa anjuran dan specs, atau arsip rilis", p, dir)
}

// pertamaYangAda mengembalikan direktori pertama yang benar-benar ada.
func pertamaYangAda(dirs []string) string {
	for _, d := range dirs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
	}
	return ""
}

// subdirJikaAda mengembalikan subdirektori bila ia ada, atau string kosong.
func subdirJikaAda(dir, nama string) string {
	p := filepath.Join(dir, nama)
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		return p
	}
	return ""
}

// findArchive mencari arsip goreleaser untuk sebuah platform.
func findArchive(dir string, p Platform) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	suffix := "_" + p.OS + "_" + p.Arch
	for _, e := range entries {
		name := e.Name()
		if !strings.Contains(name, suffix) {
			continue
		}
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") {
			return filepath.Join(dir, name)
		}
	}
	return ""
}

// extractArchive membongkar arsip rilis ke direktori sementara.
func extractArchive(arc string, p Platform) (Source, func(), error) {
	noop := func() {}
	if strings.HasSuffix(arc, ".zip") {
		return Source{}, noop, fmt.Errorf("arsip zip belum didukung sebagai sumber; pakai direktori hasil ekstraksinya")
	}

	dir, err := os.MkdirTemp("", "anjuran-src-")
	if err != nil {
		return Source{}, noop, err
	}
	cleanup := func() { os.RemoveAll(dir) }

	if err := untar(arc, dir); err != nil {
		cleanup()
		return Source{}, noop, err
	}

	bin := filepath.Join(dir, "anjuran")
	if p.OS == "windows" {
		bin = filepath.Join(dir, "anjuran.exe")
	}
	if _, err := os.Stat(bin); err != nil {
		cleanup()
		return Source{}, noop, fmt.Errorf("arsip %s tidak memuat binary anjuran", filepath.Base(arc))
	}

	specs := filepath.Join(dir, "specs")
	if fi, err := os.Stat(specs); err != nil || !fi.IsDir() {
		specs = ""
	}
	return Source{Binary: bin, Specs: specs, Origin: filepath.Base(arc)}, cleanup, nil
}

// untar membongkar tar.gz ke dalam dir.
func untar(arc, dir string) error {
	f, err := os.Open(arc)
	if err != nil {
		return err
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Entri arsip berasal dari berkas yang bisa saja disusun pihak lain;
		// path yang menunjuk keluar direktori tujuan ditolak.
		name := filepath.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}
		dst := filepath.Join(dir, name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

// errTakBisaBangun berarti mesin ini tidak punya bahan untuk membangun sendiri;
// bukan kegagalan build, melainkan ketiadaan syaratnya.
var errTakBisaBangun = errors.New("tidak ada pohon sumber atau toolchain Go")

// cariBinaryLintas mencari binary yang sudah pernah dibangun untuk platform
// lain di tempat-tempat yang wajar.
//
// Yang dicari nama berakhiran platform — anjuran-linux-arm64 — karena itulah
// yang dihasilkan `make cross` dan goreleaser. Tanpa langkah ini pengguna
// harus menyebut --from setiap kali, padahal berkasnya sudah ada di tempat
// yang bisa ditebak.
func cariBinaryLintas(p Platform) string {
	ext := ""
	if p.OS == "windows" {
		ext = ".exe"
	}
	nama := fmt.Sprintf("anjuran-%s-%s%s", p.OS, p.Arch, ext)

	kandidat := []string{"bin", "dist", "."}
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		kandidat = append(kandidat, filepath.Dir(exe))
	}
	if akar := akarSumber(); akar != "" {
		kandidat = append(kandidat, filepath.Join(akar, "bin"), filepath.Join(akar, "dist"))
	}

	for _, d := range kandidat {
		if fi, err := os.Stat(filepath.Join(d, nama)); err == nil && !fi.IsDir() {
			return d
		}
	}
	return ""
}

// bangunLintas membangun binary untuk platform tujuan dari pohon sumber.
//
// Hanya mungkin bila perintah ini memang dijalankan dari dalam pohon sumber
// anjuran dan ada toolchain Go. Binary rilis di mesin orang lain tidak punya
// keduanya, dan di sana pesan yang menyuruh memakai --from tetap benar.
func bangunLintas(p Platform, localSpecs, localExtra []string) (Source, func(), error) {
	noop := func() {}

	// Hanya platform yang memang didukung anjuran yang dicoba. Tanpa batas ini
	// sebuah host yang melaporkan dirinya plan9 akan menghasilkan kegagalan
	// kompilasi yang membingungkan, menggantikan pesan yang sebenarnya
	// menjelaskan jalan keluarnya.
	switch p.OS {
	case "linux", "darwin", "windows":
	default:
		return Source{}, noop, errTakBisaBangun
	}

	akar := akarSumber()
	if akar == "" {
		return Source{}, noop, errTakBisaBangun
	}
	if _, err := exec.LookPath("go"); err != nil {
		return Source{}, noop, errTakBisaBangun
	}

	dir, err := os.MkdirTemp("", "anjuran-lintas-")
	if err != nil {
		return Source{}, noop, err
	}
	bersihkan := func() { os.RemoveAll(dir) }

	ext := ""
	if p.OS == "windows" {
		ext = ".exe"
	}
	out := filepath.Join(dir, "anjuran"+ext)

	cmd := exec.Command("go", "build", "-o", out, "./cmd/anjuran")
	cmd.Dir = akar
	// CGO dimatikan supaya hasilnya statis dan tidak menuntut toolchain silang
	// milik OS tujuan — itulah yang membuat `go build` lintas platform berguna
	// sama sekali.
	cmd.Env = append(os.Environ(), "GOOS="+p.OS, "GOARCH="+p.Arch, "CGO_ENABLED=0")
	if keluaran, err := cmd.CombinedOutput(); err != nil {
		bersihkan()
		// Jalan keluarnya tetap disebut: build yang gagal tidak boleh
		// meninggalkan orang tanpa cara lain untuk maju.
		return Source{}, noop, fmt.Errorf(
			"membangun untuk %s gagal; bangun sendiri lalu sebutkan dengan --from: %w: %s",
			p, err, strings.TrimSpace(string(keluaran)))
	}

	specs := pertamaYangAda(append([]string{filepath.Join(akar, "specs")}, localSpecs...))
	if specs == "" {
		bersihkan()
		return Source{}, noop, fmt.Errorf("direktori spec tidak ditemukan di mesin ini; jalankan `make specs`")
	}
	return Source{
		Binary: out,
		Specs:  specs,
		Extra:  pertamaYangAda(append([]string{filepath.Join(akar, "extra")}, localExtra...)),
		Origin: "dibangun untuk " + p.String(),
	}, bersihkan, nil
}

// akarSumber menaiki direktori sampai menemukan pohon sumber anjuran.
//
// Yang diperiksa isi go.mod, bukan sekadar keberadaannya: berada di dalam
// proyek Go lain yang kebetulan mengandung direktori cmd/anjuran bukan alasan
// untuk membangun apa pun.
func akarSumber() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 40; i++ {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			if strings.Contains(string(b), "module github.com/ufhy/anjuran-cli") {
				return dir
			}
			return ""
		}
		induk := filepath.Dir(dir)
		if induk == dir {
			return ""
		}
		dir = induk
	}
	return ""
}
