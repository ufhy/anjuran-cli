// Package remote memasang anjuran di host lain lewat SSH.
//
// Yang dikirim adalah binary dan direktori spec, karena engine harus berjalan
// DI SISI REMOTE: generator seperti `kubectl get pods` hanya menjawab benar
// bila dijalankan di tempat datanya berada. Mesin lokal tidak ikut menghitung
// apa pun; ia hanya mengantarkan berkas.
package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Transport menjalankan perintah di host tujuan.
//
// Dibuat sebagai interface supaya alur bootstrap bisa diuji tanpa jaringan,
// dan supaya pengangkutan selain SSH — docker exec, kubectl exec — bisa
// ditambahkan tanpa menyentuh alurnya.
type Transport interface {
	// Exec menjalankan perintah dan mengembalikan stdout-nya.
	Exec(ctx context.Context, cmd string) (string, error)
	// Send menjalankan perintah dengan stdin dibaca dari r.
	Send(ctx context.Context, cmd string, r io.Reader) error
	// Target adalah nama host untuk ditampilkan.
	Target() string
	// Close melepas sumber daya, termasuk koneksi yang dipakai bersama.
	Close() error
}

// SSH adalah transport lewat perintah ssh milik sistem.
//
// Perintah ssh sistem dipakai dengan sengaja, bukan pustaka SSH: dengan begitu
// seluruh konfigurasi pengguna berlaku apa adanya — ProxyJump, bastion,
// Match block, agent forwarding, kunci perangkat keras. Menulis ulang semua
// itu akan menghasilkan tool yang bisa menjangkau lebih sedikit host.
type SSH struct {
	Host string
	// Args adalah opsi tambahan untuk ssh, dipakai pengujian dan kasus khusus.
	Args []string

	controlPath string
	masterOpen  bool
}

// NewSSH menyiapkan transport dengan koneksi yang dipakai bersama.
//
// Bootstrap memanggil host beberapa kali. Tanpa ControlMaster, setiap panggilan
// berarti jabat tangan baru — dan pada host yang meminta MFA, berarti pengguna
// diminta menyentuh kunci keamanannya berkali-kali untuk satu perintah.
func NewSSH(host string, args ...string) (*SSH, error) {
	dir, err := os.MkdirTemp("", "anjuran-ssh-")
	if err != nil {
		return nil, err
	}
	return &SSH{
		Host:        host,
		Args:        args,
		controlPath: filepath.Join(dir, "cm"),
	}, nil
}

func (s *SSH) Target() string { return s.Host }

// sshArgs merangkai argumen ssh lengkap dengan opsi berbagi koneksi.
func (s *SSH) sshArgs(remoteCmd string) []string {
	args := []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + s.controlPath,
		"-o", "ControlPersist=60",
		"-o", "BatchMode=yes",
	}
	args = append(args, s.Args...)
	args = append(args, s.Host, remoteCmd)
	return args
}

func (s *SSH) Exec(ctx context.Context, cmd string) (string, error) {
	var out, errb bytes.Buffer
	c := exec.CommandContext(ctx, "ssh", s.sshArgs(cmd)...)
	c.Stdout = &out
	c.Stderr = &errb
	if err := c.Run(); err != nil {
		return out.String(), wrap(err, errb.String())
	}
	s.masterOpen = true
	return out.String(), nil
}

func (s *SSH) Send(ctx context.Context, cmd string, r io.Reader) error {
	var errb bytes.Buffer
	c := exec.CommandContext(ctx, "ssh", s.sshArgs(cmd)...)
	c.Stdin = r
	c.Stdout = io.Discard
	c.Stderr = &errb
	if err := c.Run(); err != nil {
		return wrap(err, errb.String())
	}
	s.masterOpen = true
	return nil
}

// Close menutup koneksi bersama agar tidak tertinggal hidup selama
// ControlPersist.
func (s *SSH) Close() error {
	if s.masterOpen {
		args := append([]string{"-o", "ControlPath=" + s.controlPath}, s.Args...)
		args = append(args, "-O", "exit", s.Host)
		exec.Command("ssh", args...).Run()
	}
	return os.RemoveAll(filepath.Dir(s.controlPath))
}

// wrap menyertakan stderr remote ke dalam error, karena pesan dari sana jauh
// lebih berguna daripada "exit status 1".
func wrap(err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return err
	}
	// Baris terakhir biasanya yang paling menjelaskan.
	lines := strings.Split(stderr, "\n")
	return fmt.Errorf("%w: %s", err, lines[len(lines)-1])
}
