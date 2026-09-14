package remote

import (
	"context"
	"fmt"
	"strings"
)

// Platform adalah sistem operasi dan arsitektur host tujuan.
type Platform struct {
	OS   string // nilai gaya GOOS: linux, darwin
	Arch string // nilai gaya GOARCH: amd64, arm64
}

func (p Platform) String() string { return p.OS + "/" + p.Arch }

// Detect menanyakan platform host.
//
// `uname -sm` dipakai karena ada di setiap sistem mirip Unix, termasuk
// container minimal dan alat jaringan yang hanya punya busybox.
func Detect(ctx context.Context, t Transport) (Platform, error) {
	out, err := t.Exec(ctx, "uname -sm")
	if err != nil {
		return Platform{}, fmt.Errorf("mendeteksi platform %s: %w", t.Target(), err)
	}
	return parseUname(out)
}

// parseUname menerjemahkan keluaran `uname -sm` ke nilai gaya Go.
func parseUname(out string) (Platform, error) {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 {
		return Platform{}, fmt.Errorf("keluaran uname tidak dikenali: %q", strings.TrimSpace(out))
	}

	var p Platform
	switch sys := fields[0]; {
	case strings.EqualFold(sys, "Linux"):
		p.OS = "linux"
	case strings.EqualFold(sys, "Darwin"):
		p.OS = "darwin"
	case strings.HasPrefix(sys, "MINGW"), strings.HasPrefix(sys, "MSYS"),
		strings.HasPrefix(sys, "CYGWIN"):
		// Lingkungan itu menjalankan shell mirip Unix di atas Windows.
		// Binary yang dibutuhkan tetap binary Windows.
		p.OS = "windows"
	case strings.EqualFold(sys, "FreeBSD"):
		p.OS = "freebsd"
	default:
		return Platform{}, fmt.Errorf("sistem operasi %q belum didukung", sys)
	}

	switch arch := fields[1]; arch {
	case "x86_64", "amd64":
		p.Arch = "amd64"
	case "aarch64", "arm64":
		p.Arch = "arm64"
	case "i386", "i686":
		return Platform{}, fmt.Errorf("arsitektur %q sudah tidak dibangun", arch)
	default:
		return Platform{}, fmt.Errorf("arsitektur %q belum didukung", arch)
	}

	return p, nil
}

// Installed menanyakan versi uf yang sudah terpasang di host.
//
// Mengembalikan string kosong bila belum ada; itu bukan kondisi kesalahan,
// melainkan keadaan yang justru diharapkan sebelum pemasangan pertama.
func Installed(ctx context.Context, t Transport, binPath string) string {
	out, err := t.Exec(ctx, homePath(binPath)+" version 2>/dev/null || true")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// quote membungkus teks sebagai satu kata untuk shell remote, apa adanya.
//
// Kutip tunggal berarti TIDAK ADA ekspansi sama sekali — termasuk $HOME.
// Untuk path yang berpangkal di rumah pengguna, pakai homePath.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// homePath merangkai path di bawah rumah pengguna di host.
//
// Kutip GANDA dipakai dengan sengaja, supaya $HOME diekspansi oleh shell
// remote sementara sisanya tetap harfiah. Rumah pengguna tidak bisa ditebak
// dari sini: ia berbeda antara root, pengguna sistem, dan container.
func homePath(rel string) string {
	rel = strings.TrimPrefix(rel, "/")
	// Karakter yang masih punya arti di dalam kutip ganda.
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"`", "\\`",
		`$`, `\$`,
	)
	return `"$HOME/` + r.Replace(rel) + `"`
}
