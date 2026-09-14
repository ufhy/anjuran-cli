package remote

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

// localTransport menjalankan perintah lewat sh di mesin ini, dengan HOME yang
// diarahkan ke direktori sementara.
//
// Ini bukan tiruan: perintah remote yang dirakit alur bootstrap benar-benar
// dijalankan oleh shell sungguhan, termasuk ekspansi $HOME dan pengutipannya.
// Yang dihilangkan hanya jaringannya, sehingga pengujian tidak memerlukan
// sshd dan tetap menguji bagian yang paling mudah salah.
type localTransport struct {
	home string
	// log mencatat setiap perintah yang dijalankan.
	log []string
	// failOn membuat perintah yang memuat teks ini gagal.
	failOn string
}

func (l *localTransport) Target() string { return "local" }
func (l *localTransport) Close() error   { return nil }

func (l *localTransport) run(ctx context.Context, cmd string, stdin io.Reader) (string, error) {
	l.log = append(l.log, cmd)
	if l.failOn != "" && contains(cmd, l.failOn) {
		return "", exec.ErrNotFound
	}
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	c.Env = []string{"HOME=" + l.home, "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	c.Stdin = stdin
	var out bytes.Buffer
	c.Stdout = &out
	err := c.Run()
	return out.String(), err
}

func (l *localTransport) Exec(ctx context.Context, cmd string) (string, error) {
	return l.run(ctx, cmd, nil)
}

func (l *localTransport) Send(ctx context.Context, cmd string, r io.Reader) error {
	_, err := l.run(ctx, cmd, r)
	return err
}
