package main

import (
	"html/template"
	"strings"
	"testing"
)

func TestHideHiddenLines(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`<p>Wstęp</p><p>A: {{.Field "a"}}</p><p>Koniec</p>`,
			`<p>Wstęp</p>{{if .Shown "a"}}<p>A: {{.Field "a"}}</p>{{end}}<p>Koniec</p>`},
		// lista pod linijką znika razem z nią, jej punkty z polami osobno
		{`<p>A: {{.Field "a"}}</p><ul><li>przynieść</li><li>B: {{ .FieldOr "b" "nie" }}</li></ul><ul><li>zostaje</li></ul>`,
			`{{if .Shown "a"}}<p>A: {{.Field "a"}}</p><ul><li>przynieść</li>{{if .Shown "b"}}<li>B: {{ .FieldOr "b" "nie" }}</li>{{end}}</ul>{{end}}<ul><li>zostaje</li></ul>`},
		{`<h3>{{.MTotal "x"}} i {{.Field "y"}}</h3>`, `{{if .Shown "x" "y"}}<h3>{{.MTotal "x"}} i {{.Field "y"}}</h3>{{end}}`},
		// niedomknięty {{if}} - linijka bez zmian
		{`<p>{{if .Field "a"}}tak</p><p>{{end}}</p>`, `<p>{{if .Field "a"}}tak</p><p>{{end}}</p>`},
		// własny warunek admina - jego sprawa
		{`<p>{{if .Field "a"}}tak{{end}}</p>`, `<p>{{if .Field "a"}}tak{{end}}</p>`},
		{`<p>{{if true}}{{.Field "a"}}{{end}}</p>`, `{{if .Shown "a"}}<p>{{if true}}{{.Field "a"}}{{end}}</p>{{end}}`},
		{`<p>{{.Name}}</p>`, `<p>{{.Name}}</p>`},
		{`Bez akapitów {{.Field "a"}}`, `Bez akapitów {{.Field "a"}}`},
		{`<table><tr><td>A</td><td>{{.Field "a"}}</td></tr></table>`,
			`<table>{{if .Shown "a"}}<tr><td>A</td><td>{{.Field "a"}}</td></tr>{{end}}</table>`},
	} {
		if got := hideHiddenLines(c.in); got != c.want {
			t.Errorf("hideHiddenLines(%s)\n got %s\nwant %s", c.in, got, c.want)
		}
	}
}

// Mail formularza z sekcjami zależnymi od klasy: gość z klasy 2 nie dostaje
// linijek pól klasy 1.
func TestFormMailConds(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	content := `[
	 {"type":"text","name":"name-REQUIRED","label":"Imię"},
	 {"type":"text","name":"surname-REQUIRED","label":"Nazwisko"},
	 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail"},
	 {"type":"select","name":"klasa","label":"Klasa","values":[{"label":"1","value":"1"},{"label":"2","value":"2"}]},
	 {"type":"header","label":"Klasa 1","show_if":{"field":"klasa","values":["1"]}},
	 {"type":"checkbox-group","name":"k1","label":"<b>Sala</b> 1","values":[{"label":"tak","value":"tak"}]},
	 {"type":"header","label":"Klasa 2","show_if":{"field":"klasa","values":["2"]}},
	 {"type":"checkbox-group","name":"k2","label":"<b>Sala</b> 2","values":[{"label":"tak","value":"tak"}]},
	 {"type":"text","name":"k2b","label":"Ciasto 2"},
	 {"type":"header","label":"Uwagi"},
	 {"type":"textarea","name":"uwagi","label":"Uwagi"}
	]`
	templ := FormTemplate{Name: "Bal", URL: "bal", Content: ToNS(content), UserID: userID, CreatedDate: 1, Language: "pl"}
	templ.ID = must(t)(db.FormTemplateAdd(&templ))
	formID := must(t)(db.FormAdd(&Form{Name: ToNS("Łucja"), Surname: ToNS("Nowak"), Email: ToNS("l@example.com"),
		UserID: userID, FormTemplateID: templ.ID, CreatedDate: 1}))
	for _, name := range []string{"klasa", "k1", "k2", "k2b", "uwagi"} {
		val := map[string]string{"klasa": "2", "k2": "tak", "uwagi": "a <b>\nb"}[name]
		fid := must(t)(db.FormFieldAdd(&FormField{Name: name, Display: name, Type: "text", FormTemplateID: templ.ID}))
		must(t)(db.FormAnswerAdd(&FormAnswer{Value: ToNS(val), FormFieldID: fid, FormID: formID}))
	}

	mail := `<p>Wstęp</p><p>Funkcja klasa 1: {{.Field "k1"}}</p><ul><li>o 7:00</li></ul>` +
		`<p>Funkcja klasa 2: {{.Field "k2"}}</p><p>Ciasto: {{.Field "k2b"}}</p><p>Koniec</p><p>{{.Answers}}</p>`
	text, _ := parseFormMailTemplate("Łucja", "Nowak", mail, formID, User{ID: userID}, templ, db)
	for _, no := range []string{"klasa 1", "o 7:00", "Klasa 1"} {
		if strings.Contains(text, no) {
			t.Errorf("mail z %q: %s", no, text)
		}
	}
	for _, want := range []string{"Wstęp", "Funkcja klasa 2: tak", "Ciasto: </p>", "Koniec",
		"<strong>Klasa 2</strong><br>Sala 2: <strong>tak</strong><br>", "Uwagi: <strong>a &lt;b&gt;<br>b</strong>"} {
		if !strings.Contains(text, want) {
			t.Errorf("mail bez %q: %s", want, text)
		}
	}
	// pole widoczne, ale puste, nie trafia do .Answers
	if strings.Contains(text, "Ciasto 2") {
		t.Errorf(".Answers z pustym polem: %s", text)
	}

	ff := &FormFuncs{DB: db, User: User{ID: userID}, Template: templ, FormID: formID}
	if ff.Shown("k1") || !ff.Shown("k2") || !ff.Shown("k1", "k2") || !ff.Shown("nieznane") || ff.Shown("Sala 1") {
		t.Error("Shown: zła widoczność")
	}
	if !(&FormFuncs{DB: db, Template: templ}).Shown("k1") {
		t.Error("Shown bez zgłoszenia ma pokazywać wszystko")
	}

	// propozycja maila dla formularza z warunkami: odpowiedzi przez .Answers,
	// bez nagłówków sekcji, które zostałyby w mailu każdego gościa
	_, draft := formMailDraft(db, User{ID: userID}, templ, true, "pl")
	if !strings.Contains(draft, "{{.Answers}}") || strings.Contains(draft, "Klasa 1") {
		t.Errorf("propozycja: %s", draft)
	}
	if _, err := template.New("x").Parse(draft); err != nil {
		t.Error(err)
	}
}
