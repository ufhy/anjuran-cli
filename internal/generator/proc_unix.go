//go:build !windows

package generator

import (
	"os/exec"
	"syscall"
)

// setProcessGroup menaruh proses anak di grup proses miliknya sendiri.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup mematikan seluruh grup proses, bukan hanya anak langsungnya.
//
// Tanpa ini, generator yang memanggil program lain akan meninggalkan cucu
// proses yang tetap hidup setelah waktunya habis — pada tombol yang ditekan
// puluhan kali per menit, itu menumpuk.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
