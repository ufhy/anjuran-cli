package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// EnvLog turns on recording of the bytes drawn to the terminal.
const EnvLog = "ANJURAN_LOG"

// record wraps the draw destination so every byte sent to the terminal is
// also copied to a file.
//
// It exists for one purpose: investigating "the screen looks weird" reports
// from someone else's terminal. Symptoms like that cannot always be
// reproduced — shell configuration, prompt theme, window size, and any other
// tool that also draws all play a part. Asking the user to record with
// `script` adds one more PTY layer, and that layer can itself change the
// symptom.
//
// Only what anjuran WRITES is recorded; what the shell writes is invisible
// here. That is precisely what we want: it answers "was it anjuran that
// erased it" without capturing anything from the user's screen beyond
// anjuran's own box.
//
// The second return value closes the file, and must always be called.
// Leaving it open until the process exits looks safe on Unix, but on Windows
// a file still held open cannot be deleted or moved by anyone — including by
// the user trying to clean up their own recording.
func record(w io.Writer) (io.Writer, func()) {
	path := os.Getenv(EnvLog)
	if path == "" {
		return w, func() {}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		// Recording is a diagnostic aid; its failure must never get in the
		// way of the actual work.
		return w, func() {}
	}
	fmt.Fprintf(f, "\n--- %s pid=%d ---\n", time.Now().Format("15:04:05.000"), os.Getpid())
	return io.MultiWriter(w, &asText{f}), func() { f.Close() }
}

// asText writes raw bytes in a form the eye can read, so escape sequences
// appear as themselves instead of being executed by whichever terminal opens
// the file.
type asText struct{ w io.Writer }

func (t *asText) Write(p []byte) (int, error) {
	var b []byte
	for _, c := range p {
		switch {
		case c == 0x1b:
			b = append(b, []byte("<ESC>")...)
		case c == '\r':
			b = append(b, []byte("<CR>")...)
		case c == '\n':
			b = append(b, []byte("<LF>\n")...)
		case c == '\b':
			b = append(b, []byte("<BS>")...)
		case c == 0x07:
			b = append(b, []byte("<BEL>")...)
		case c < 0x20:
			b = append(b, []byte(fmt.Sprintf("<%02x>", c))...)
		default:
			b = append(b, c)
		}
	}
	if _, err := t.w.Write(b); err != nil {
		return 0, err
	}
	return len(p), nil
}
