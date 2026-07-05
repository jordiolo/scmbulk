package cmd

import (
	"os"
	"testing"
)

func withStdin(t *testing.T, feed func(w *os.File)) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	feed(w)
}

func TestConfirmStdinClosedWithNoInputDeclines(t *testing.T) {
	withStdin(t, func(w *os.File) { w.Close() }) // stdin closed before any input arrives
	if confirmStdin("continue?") {
		t.Fatal("expected false when stdin is closed with no input")
	}
}

func TestConfirmStdinEmptyLineAccepts(t *testing.T) {
	withStdin(t, func(w *os.File) {
		w.WriteString("\n")
		w.Close()
	})
	if !confirmStdin("continue?") {
		t.Fatal("expected true on empty line (default yes)")
	}
}

func TestConfirmStdinNoAnswerDeclines(t *testing.T) {
	withStdin(t, func(w *os.File) {
		w.WriteString("n\n")
		w.Close()
	})
	if confirmStdin("continue?") {
		t.Fatal("expected false on explicit 'n'")
	}
}
