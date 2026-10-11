package main

import "testing"

// Treść kodu QR płatności (SPAYD) - ten sam napis, który czytają aplikacje banków.
func TestSPAYD(t *testing.T) {
	q := NewQRPayment("CZ6508000000192000145399", "Steelhouse s.r.o.", 450, "CZK", "Bal_Nowak_Łucja", 123)
	got, err := q.spayd()
	want := "SPD*1.0*ACC:CZ6508000000192000145399*AM:450.00*CC:CZK*MSG:BAL_NOWAK_LUCJA*RN:STEELHOUSE S.R.O.*X-VS:123*"
	if err != nil || got != want {
		t.Errorf("spayd = %q, %v\nwant    %q", got, err, want)
	}

	q = NewQRPayment("CZ6508000000192000145399", "", 1200, "", "", 0)
	if got, _ := q.spayd(); got != "SPD*1.0*ACC:CZ6508000000192000145399*AM:1200.00*X-VS:0*" {
		t.Errorf("spayd without optional parts = %q", got)
	}

	if _, err := NewQRPayment("CZ65", "", 1, "CZK", "", 0).spayd(); err == nil {
		t.Error("too short account accepted")
	}
}
