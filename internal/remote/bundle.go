package remote

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
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
)

// Bundle menulis arsip tar.gz berisi binary dan spec.
//
// Arsip dialirkan langsung ke stdin perintah remote, tanpa pernah menjadi
// berkas di mesin lokal: satu proses, satu koneksi, tanpa sisa.
func Bundle(binPath, specsDir string, w io.Writer) (int64, error) {
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
	// Origin menjelaskan asalnya untuk ditampilkan.
	Origin string
}

// ResolveSource menentukan berkas apa yang dikirim untuk sebuah platform.
//
// from boleh kosong, sebuah direktori berisi anjuran dan specs, atau direktori
// hasil rilis yang memuat arsip goreleaser.
func ResolveSource(from string, p Platform, localSpecs []string) (Source, func(), error) {
	noop := func() {}

	if from != "" {
		return fromDir(from, p, noop)
	}

	// Tanpa asal yang disebut, satu-satunya yang bisa dipakai adalah binary
	// yang sedang berjalan — dan itu hanya cocok bila platformnya sama.
	if p.OS != runtime.GOOS || p.Arch != runtime.GOARCH {
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
	return Source{Binary: exe, Specs: specs, Origin: "binary anjuran yang sedang berjalan"}, noop, nil
}

// fromDir mencari sumber di dalam sebuah direktori.
func fromDir(dir string, p Platform, noop func()) (Source, func(), error) {
	// Bentuk pertama: direktori yang sudah berisi anjuran dan specs.
	bin := filepath.Join(dir, "anjuran")
	if p.OS == "windows" {
		bin = filepath.Join(dir, "anjuran.exe")
	}
	if fi, err := os.Stat(bin); err == nil && !fi.IsDir() {
		specs := filepath.Join(dir, "specs")
		if fi, err := os.Stat(specs); err != nil || !fi.IsDir() {
			specs = ""
		}
		return Source{Binary: bin, Specs: specs, Origin: dir}, noop, nil
	}

	// Bentuk kedua: direktori hasil rilis berisi arsip per platform.
	if arc := findArchive(dir, p); arc != "" {
		return extractArchive(arc, p)
	}

	return Source{}, noop, fmt.Errorf(
		"tidak menemukan anjuran untuk %s di %s; isinya harus berupa anjuran dan specs, atau arsip rilis", p, dir)
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
