package main

import (
	"encoding/csv"
	"io"
	"regexp"
)

// safeCSV - CSV do otwarcia w Excelu/LibreOffice. Komórki z danymi gości
// zaczynające się od = + - @ arkusz potraktowałby jako formułę (np.
// =HYPERLINK(...) w imieniu z formularza), więc dostają z przodu apostrof.
// Zwykłe liczby i telefony ("-5", "+420 777 123 456") zostają bez zmian.
type safeCSV struct{ *csv.Writer }

func newSafeCSV(w io.Writer) safeCSV { return safeCSV{csv.NewWriter(w)} }

var csvPlainNumber = regexp.MustCompile(`^[+-]?[0-9][0-9 .,]*$`)

func csvCell(s string) string {
	if s == "" || csvPlainNumber.MatchString(s) {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func (c safeCSV) Write(rec []string) error {
	out := make([]string, len(rec))
	for i, s := range rec {
		out[i] = csvCell(s)
	}
	return c.Writer.Write(out)
}
