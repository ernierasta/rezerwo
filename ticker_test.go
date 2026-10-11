package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Stare kody QR i bilety PDF znikają, reszta mediów organizacji zostaje.
func TestRemoveGeneratedFiles(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-10 * 24 * time.Hour)
	write := func(p string, mod time.Time) string {
		t.Helper()
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(full, mod, mod); err != nil {
			t.Fatal(err)
		}
		return full
	}
	gone := []string{write("org/qrcode/a.jpeg", old), write("org/pdf/one/b.pdf", old), write("org/pdf/all/c.pdf", old)}
	kept := []string{write("org/qrcode/new.jpeg", time.Now()), write("org/banner.png", old), write("org/email/logo.png", old), write("fonts/x.ttf", old)}

	if n := RemoveGeneratedFiles(root, time.Now().Add(-GENERATEDMAXAGE)); n != 3 {
		t.Errorf("removed %d, want 3", n)
	}
	for _, p := range gone {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s still there", p)
		}
	}
	for _, p := range kept {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s removed", p)
		}
	}
}
