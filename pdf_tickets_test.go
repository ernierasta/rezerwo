package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Bilety PDF: arkusz A4 (4 na stronę) i pojedyncze bilety. Czcionki leżą
// w media/fonts (poza repozytorium), bez nich test jest pomijany.
func TestGenerateTicketsPDF(t *testing.T) {
	fonts, err := filepath.Abs(FontsDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(fonts, "NotoSans-Bold.ttf")); err != nil {
		t.Skip("no fonts in media/fonts")
	}
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("media", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fonts, FontsDir); err != nil {
		t.Fatal(err)
	}

	files, err := GenerateTicketsPDF(&TicketParams{
		UserName:  "Macierz Szkolna",
		UserURL:   "test",
		EventName: "Bal 2027",
		SeatNrs:   []int{1, 2, 3, 4, 5},
		Rooms:     []string{"Sala", "Sala", "Sala", "Balkon", "Balkon"},
		Lang:      "cs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 6 || files[0] != "media/test/pdf/all/Bal_2027_1_5.pdf" || files[5] != "media/test/pdf/one/Bal_2027_5.pdf" {
		t.Fatalf("files: %v", files)
	}
	all, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(all, []byte("/Type /Page\n")); n != 2 {
		t.Errorf("A4 sheet has %d pages, want 2", n)
	}
	one, err := os.ReadFile(files[1])
	if err != nil || bytes.Count(one, []byte("/Type /Page\n")) != 1 {
		t.Errorf("single ticket: %v", err)
	}
}
