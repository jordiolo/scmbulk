package cmd

import (
	"os"
	"testing"

	"scmbulk/pkg/runner"
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

func TestConfirmErrorStdinClosedAborts(t *testing.T) {
	withStdin(t, func(w *os.File) { w.Close() })
	if got := confirmErrorStdin("error occurred; continue?"); got != runner.ActionAbort {
		t.Fatalf("expected ActionAbort when stdin is closed with no input, got %v", got)
	}
}

func TestConfirmErrorStdinEmptyLineContinues(t *testing.T) {
	withStdin(t, func(w *os.File) {
		w.WriteString("\n")
		w.Close()
	})
	if got := confirmErrorStdin("error occurred; continue?"); got != runner.ActionContinue {
		t.Fatalf("expected ActionContinue on empty line, got %v", got)
	}
}

func TestConfirmErrorStdinRetries(t *testing.T) {
	withStdin(t, func(w *os.File) {
		w.WriteString("r\n")
		w.Close()
	})
	if got := confirmErrorStdin("error occurred; continue?"); got != runner.ActionRetry {
		t.Fatalf("expected ActionRetry on 'r', got %v", got)
	}
}

func TestConfirmErrorStdinAborts(t *testing.T) {
	withStdin(t, func(w *os.File) {
		w.WriteString("a\n")
		w.Close()
	})
	if got := confirmErrorStdin("error occurred; continue?"); got != runner.ActionAbort {
		t.Fatalf("expected ActionAbort on 'a', got %v", got)
	}
}
