//go:build windows

package generator

import "os/exec"

// Windows tidak mengenal grup proses gaya POSIX. CommandContext sudah
// mematikan proses anaknya saat konteks berakhir; keturunannya menjadi
// tanggung jawab job object, yang belum diperlukan di sini.
func setProcessGroup(*exec.Cmd) {}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}
