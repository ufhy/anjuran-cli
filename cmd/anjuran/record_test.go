package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Escape sequences must read as text, not be executed by whichever terminal
// happens to open the recording.
func TestRecordWritesEscapesAsText(t *testing.T) {
	var b strings.Builder
	w := &asText{&b}

	n, err := w.Write([]byte("a\x1b[2mb\r\n\bc\x07\x00"))
	if err != nil {
		t.Fatal(err)
	}
	// The reported length is the length of the INPUT, not of what was
	// written; io.MultiWriter checks it and treats a mismatch as a failed
	// write.
	if n != len("a\x1b[2mb\r\n\bc\x07\x00") {
		t.Errorf("n = %d, want %d", n, len("a\x1b[2mb\r\n\bc\x07\x00"))
	}
	for _, want := range []string{"<ESC>", "<CR>", "<LF>", "<BS>", "<BEL>", "<00>"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("recording does not contain %q: %q", want, b.String())
		}
	}
}

// Without ANJURAN_LOG the draw destination is returned untouched: recording
// must not add weight to the ordinary path.
func TestRecordOffWithoutEnv(t *testing.T) {
	t.Setenv(EnvLog, "")
	var b strings.Builder
	got, closeFn := record(&b)
	defer closeFn()
	if got != (&b) {
		t.Error("without ANJURAN_LOG the writer must be returned untouched")
	}
}

func TestRecordWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.log")
	t.Setenv(EnvLog, path)

	var b strings.Builder
	w, closeFn := record(&b)
	if _, err := w.Write([]byte("halo\x1b[K")); err != nil {
		t.Fatal(err)
	}
	// Closed before the file is read and before t.TempDir cleans it up:
	// Windows refuses to delete a file a process still holds open.
	closeFn()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "halo<ESC>[K") {
		t.Errorf("recording file = %q", content)
	}
	// The terminal still receives the original bytes, not the readable form.
	if b.String() != "halo\x1b[K" {
		t.Errorf("terminal received %q, want the raw bytes", b.String())
	}
}

// A recording file that cannot be opened must not get in the way of the work.
func TestRecordFailureDoesNotBlock(t *testing.T) {
	t.Setenv(EnvLog, filepath.Join(t.TempDir(), "does-not-exist", "a.log"))
	var b strings.Builder
	got, closeFn := record(&b)
	defer closeFn()
	if got != (&b) {
		t.Error("failing to open the file must return the writer untouched")
	}
}

// The file must genuinely be CLOSED: on Windows a file still held open cannot
// be deleted by anyone, including by the user trying to clean up their own
// recording.
func TestRecordClosesItsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.log")
	t.Setenv(EnvLog, path)

	var b strings.Builder
	w, closeFn := record(&b)
	if _, err := w.Write([]byte("halo")); err != nil {
		t.Fatal(err)
	}
	closeFn()

	// Deleting it is the most direct way to ask "is it still held open?" —
	// and on Windows it is the only way that truly answers.
	if err := os.Remove(path); err != nil {
		t.Errorf("recording file still held open after close: %v", err)
	}
}
