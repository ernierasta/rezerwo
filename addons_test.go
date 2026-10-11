package main

import "testing"

// Definicja w formacie, w jakim zapisuje ją formBuilder.
const addonsContent = `[
	{"type":"header","subtype":"h3","label":"Wyżywienie"},
	{"type":"select","label":"Obiad","name":"obiad","required":true,
	 "values":[{"label":"Mięsny","value":"miesny","selected":true},
	           {"label":"Wegetariański","value":"wege","selected":false}]},
	{"type":"multiplierField","label":"Dodatkowa <b>porcja</b>","name":"porcja","multipl":"85","after":"Kč"},
	{"type":"text","label":"Godzina przyjazdu","name":"godzina"},
	{"type":"select","label":"Kolacja","name":"kolacja-perseat",
	 "values":[{"label":"Mięsna","value":"miesna"},{"label":"Wegetariańska","value":"wege"}]}
]`

func TestParseAddonFields(t *testing.T) {
	ff := ParseAddonFields(addonsContent)
	if len(ff) != 4 {
		t.Fatalf("oczekiwano 4 pól (nagłówek pomijany), jest %d: %+v", len(ff), ff)
	}

	if ff[0].Name != "obiad" || ff[0].Type != "select" || !ff[0].Required {
		t.Errorf("pole wyboru wczytane źle: %+v", ff[0])
	}
	if len(ff[0].Options) != 2 || ff[0].Options[0].Value != "miesny" || !ff[0].Options[0].Selected {
		t.Errorf("opcje listy wczytane źle: %+v", ff[0].Options)
	}

	if ff[1].Price != 85 || !ff[1].IsPriced() {
		t.Errorf("cena pola wielokrotności wczytana źle: %+v", ff[1])
	}
	if ff[1].Label != "Dodatkowa porcja" {
		t.Errorf("etykieta powinna być bez HTML, jest %q", ff[1].Label)
	}
	if ff[2].IsPriced() {
		t.Errorf("zwykłe pole tekstowe nie może być płatne: %+v", ff[2])
	}

	if !ff[3].PerSeat {
		t.Errorf("pole z końcówką %q ma być powtarzane wg liczby miejsc: %+v", PERSEATSUFFIX, ff[3])
	}
	if ff[0].PerSeat {
		t.Errorf("zwykłe pole nie może być powtarzane: %+v", ff[0])
	}
}

func TestCollectAddonAnswers(t *testing.T) {
	ff := ParseAddonFields(addonsContent)

	form := map[string][]string{
		"addon-obiad":         {"wege"},
		"addon-porcja-amount": {"2"},
		"addon-godzina":       {"  "}, // puste - pomijamy
	}

	aa := CollectAddonAnswers(ff, form, 0)
	if len(aa) != 2 {
		t.Fatalf("oczekiwano 2 odpowiedzi, jest %d: %+v", len(aa), aa)
	}
	if aa[0].Value != "wege" || aa[0].Sum() != 0 {
		t.Errorf("odpowiedź wyboru źle policzona: %+v", aa[0])
	}
	if aa[1].Amount != 2 || aa[1].Sum() != 170 {
		t.Errorf("dopłata źle policzona: %+v, suma %d", aa[1], aa[1].Sum())
	}
	if got, want := aa[1].Description(), "Dodatkowa porcja: 2 × 85"; got != want {
		t.Errorf("opis pozycji: %q, oczekiwano %q", got, want)
	}
}

// Pole "tyle razy, ile miejsc" wypełnia się osobno dla każdego miejsca.
func TestCollectAddonAnswersPerSeat(t *testing.T) {
	ff := ParseAddonFields(addonsContent)

	form := map[string][]string{
		"addon-kolacja-perseat-1": {"miesna"},
		"addon-kolacja-perseat-2": {"wege"},
		"addon-kolacja-perseat-3": {""}, // trzeciego nie wybrano
	}

	aa := CollectAddonAnswers(ff, form, 3)
	if len(aa) != 2 {
		t.Fatalf("oczekiwano 2 odpowiedzi, jest %d: %+v", len(aa), aa)
	}
	if aa[0].Repeat != 1 || aa[0].Value != "miesna" {
		t.Errorf("pierwsze powtórzenie źle: %+v", aa[0])
	}
	if aa[1].Repeat != 2 || aa[1].Value != "wege" {
		t.Errorf("drugie powtórzenie źle: %+v", aa[1])
	}
	if got, want := aa[1].Description(), "Kolacja 2: wege"; got != want {
		t.Errorf("opis powtórzenia: %q, oczekiwano %q", got, want)
	}
	if got, want := AddonInputName(ff[3], 2), "addon-kolacja-perseat-2"; got != want {
		t.Errorf("nazwa pola: %q, oczekiwano %q", got, want)
	}
}

func TestCollectAddonAnswersEmpty(t *testing.T) {
	ff := ParseAddonFields(addonsContent)

	// nic nie wybrano, zero sztuk - zamówienie bez dodatków
	aa := CollectAddonAnswers(ff, map[string][]string{"addon-porcja-amount": {"0"}}, 2)
	if len(aa) != 0 {
		t.Errorf("oczekiwano braku odpowiedzi, jest %+v", aa)
	}
}

func TestParseAddonFieldsBroken(t *testing.T) {
	if ff := ParseAddonFields(""); ff != nil {
		t.Errorf("pusta definicja ma dawać nil, jest %+v", ff)
	}
	if ff := ParseAddonFields("{niepoprawny json"); ff != nil {
		t.Errorf("zepsuty JSON ma dawać nil, jest %+v", ff)
	}
}
