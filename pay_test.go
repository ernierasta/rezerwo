package main

// Testy danych do przelewu (pay.go). Uruchomienie: go test -run 'TestCzechAccount|TestPayInfo'

import (
	"strings"
	"testing"
)

func TestCzechAccount(t *testing.T) {
	cases := map[string]string{
		"CZ6508000000192000145399":     "19-2000145399/0800",
		"CZ4327000000001588355009":     "1588355009/2700",
		"PL61109010140000071219812874": "",
		"CZ65080000001920001453":       "",
	}
	for iban, want := range cases {
		if got := czechAccount(iban); got != want {
			t.Errorf("czechAccount(%q) = %q, chcę %q", iban, got, want)
		}
	}

	iban, bic := splitIBAN(" cz07 2010 0000 0029 0259 0133+FIOBCZPPXXX")
	if iban != "CZ0720100000002902590133" || bic != "FIOBCZPPXXX" {
		t.Errorf("splitIBAN = %q, %q", iban, bic)
	}
	if got := groupIBAN("CZ0720100000002902590133"); got != "CZ07 2010 0000 0029 0259 0133" {
		t.Errorf("groupIBAN = %q", got)
	}
}

func TestPayInfo(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := must(t)(db.UserAdd(&User{Email: "org@example.com", URL: "org"}))
	must(t)(db.BankAccountAdd(&BankAccount{
		Name:          "Bilety",
		IBAN:          "CZ0720100000002902590133+FIOBCZPPXXX",
		RecipientName: ToNS("Macierz"),
		Currency:      "CZK",
		VarSymbol:     ToNI(2026),
		UserID:        userID,
	}))

	of := &OrderFuncs{Name: "Łucja", Surname: "Nowak", TotalPrice: "1200", DB: db, User: User{ID: userID}, Lang: "pl", payPrefix: "bal2026"}
	got := string(of.PayInfo("Bilety"))
	for _, want := range []string{"Macierz", "2902590133/2010", FormatMoney(1200, "CZK"), "2026", "BAL2026_NOWAK_LUCJA", "Variabilní symbol"} {
		if !strings.Contains(got, want) {
			t.Errorf("PayInfo bez %q: %s", want, got)
		}
	}
	// konta są czeskie - IBAN tylko w {{.PayIBAN}}
	for _, bad := range []string{"CZ07", "FIOBCZPPXXX"} {
		if strings.Contains(got, bad) {
			t.Errorf("PayInfo z %q: %s", bad, got)
		}
	}

	if got := string(of.PayMessage("Bilety")); got != "BAL2026_NOWAK_LUCJA" {
		t.Errorf("PayMessage = %q", got)
	}
	if got := string(of.PayVS("Bilety")); got != "2026" {
		t.Errorf("PayVS = %q", got)
	}

	// nieznane konto: w mailu nic, w podglądzie znacznik
	if got := of.PayInfo("Brak"); got != "" {
		t.Errorf("PayInfo bez konta = %q", got)
	}
	of.Preview = true
	if got := string(of.PayInfo("Brak")); !strings.Contains(got, "Dane do przelewu") {
		t.Errorf("podgląd bez konta = %q", got)
	}
}

func TestPayMessage(t *testing.T) {
	if got := payMessage("", "Nowak_Łucja"); got != "NOWAK_LUCJA" {
		t.Errorf("payMessage bez początku = %q", got)
	}
	long := payMessage(strings.Repeat("a", 50), "Nowak_Łucja")
	if len(long) != 60 {
		t.Errorf("payMessage nie skrócona do 60 znaków: %q (%d)", long, len(long))
	}
	if got := payMessage(" ", "Nowak_Jan"); got != "NOWAK_JAN" {
		t.Errorf("payMessage z pustym początkiem = %q", got)
	}
}

func TestIBANValid(t *testing.T) {
	for iban, want := range map[string]bool{
		"CZ6508000000192000145399":     true,
		"CZ0720100000002902590133":     true,
		"PL61109010140000071219812874": true,
		"CZ6508000000192000145398":     false,
		"CZ65":                         false,
	} {
		if got := ibanValid(iban); got != want {
			t.Errorf("ibanValid(%q) = %v", iban, got)
		}
	}
}
