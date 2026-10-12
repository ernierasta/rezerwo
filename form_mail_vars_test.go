package main

import (
	"strings"
	"testing"
)

// Zmienne maili formularzy (formMailVars) i ich wartości (FormFuncs).
func TestFormMailVars(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	must(t)(db.BankAccountAdd(&BankAccount{Name: "Bilety", IBAN: "CZ0720100000002902590133", Currency: "CZK", UserID: userID}))
	content := `[
	 {"type":"text","name":"name-REQUIRED","label":"Imię"},
	 {"type":"text","name":"surname-REQUIRED","label":"Nazwisko"},
	 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail"},
	 {"type":"header","label":"Sekcja"},
	 {"type":"paragraph","label":"Opis"},
	 {"type":"text","name":"klasa","label":"<b>Klasa</b>"},
	 {"type":"multiplierField","name":"bilety","label":"Bilety","multipl":"300"}
	]`
	templ := FormTemplate{Name: "Bal", URL: "bal", Content: ToNS(content), UserID: userID, CreatedDate: 1,
		MoneyAmountFieldName: ToNS("bilety"), NotificationID: ToNI(7), Language: "pl", PayMessage: "bal"}
	templ.ID = must(t)(db.FormTemplateAdd(&templ))

	vars := formMailVars(templ)
	var fields []string
	for _, v := range vars {
		if v.Code == ".Field" && !v.Hidden {
			fields = append(fields, v.Arg+"="+v.Text)
		}
	}
	if got := strings.Join(fields, ","); got != "klasa=Klasa,bilety=Bilety" {
		t.Errorf("pola maila = %q", got)
	}

	formID := must(t)(db.FormAdd(&Form{Name: ToNS("Łucja"), Surname: ToNS("Nowak"), Email: ToNS("l@example.com"),
		UserID: userID, FormTemplateID: templ.ID, CreatedDate: 1}))
	for name, val := range map[string]string{"klasa": "3B", "bilety": "2 * 300 = 600"} {
		fid := must(t)(db.FormFieldAdd(&FormField{Name: name, Display: name, Type: "text", FormTemplateID: templ.ID}))
		must(t)(db.FormAnswerAdd(&FormAnswer{Value: ToNS(val), FormFieldID: fid, FormID: formID}))
	}

	subject := formMailSubject("Łucja", "Nowak", `Bal: {{.Name}} {{.Surname}}, {{.Field "klasa"}}, {{.TotalPrice}}`, formID, User{ID: userID}, templ, db)
	if subject != "Bal: Łucja Nowak, 3B, 600" {
		t.Errorf("temat = %q", subject)
	}
	text, _ := parseFormMailTemplate("Łucja", "Nowak", `{{.Email}} {{.PayInfo "Bilety"}}`, formID, User{ID: userID}, templ, db)
	for _, want := range []string{"l@example.com", "2902590133/2010", FormatMoney(600, "CZK"), "BAL_NOWAK_LUCJA"} {
		if !strings.Contains(text, want) {
			t.Errorf("treść bez %q: %s", want, text)
		}
	}
}

func TestFormMailDraft(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	accID := must(t)(db.BankAccountAdd(&BankAccount{Name: "Bilety", IBAN: "CZ0720100000002902590133", Currency: "CZK", UserID: userID}))
	templ := FormTemplate{Name: "Bal", URL: "bal", UserID: userID, CreatedDate: 1, Language: "pl",
		MoneyAmountFieldName: ToNS("bilety"), BankAccountID: ToNI(accID),
		Content: ToNS(`[{"type":"text","name":"name-REQUIRED","label":"Imię"},{"type":"header","label":"Sekcja"},
		 {"type":"text","name":"klasa","label":"Klasa"},{"type":"multiplierField","name":"bilety","label":"Bilety","multipl":"300"}]`)}
	templ.ID = must(t)(db.FormTemplateAdd(&templ))
	user := User{ID: userID}

	subject, text := formMailDraft(db, user, templ, false, "pl")
	if err := checkMailTemplate(subject, text); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`{{.Field "klasa"}}`, `{{.Field "bilety"}}`, "<strong>Sekcja</strong>", `{{.PayInfo "Bilety"}}`, `{{.QRPayMail "Bilety"}}`} {
		if !strings.Contains(text, want) {
			t.Errorf("szkic bez %q: %s", want, text)
		}
	}
	if strings.Contains(text, "name-REQUIRED") {
		t.Errorf("szkic z polem imienia: %s", text)
	}
	_, admin := formMailDraft(db, user, templ, true, "pl")
	if strings.Contains(admin, "PayInfo") || !strings.Contains(admin, "{{.TotalPrice}}") {
		t.Errorf("szkic dla administratora: %s", admin)
	}

	ps, ph := formMailPreview(templ, "pl", subject+" {{.Field \"klasa\"}}", text+`{{.Sum "klasa"}}{{if .Name}}x{{end}}`)
	if !strings.Contains(ps, "[Klasa]") || !strings.Contains(ph, ">Klasa<") || strings.Contains(ph, "{{") {
		t.Errorf("podgląd: %q / %s", ps, ph)
	}

	// maile do wyboru na stronie formularza: własne i nieprzypisane
	other := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Inny", URL: "inny", UserID: userID, CreatedDate: 1}))
	own := must(t)(db.NotificationAdd(&Notification{Name: "własny", Type: "email", RelatedTo: "forms", UserID: userID, FormTemplateID: ToNI(templ.ID)}))
	free := must(t)(db.NotificationAdd(&Notification{Name: "wolny", Type: "email", RelatedTo: "forms", UserID: userID}))
	must(t)(db.NotificationAdd(&Notification{Name: "cudzy", Type: "email", RelatedTo: "forms", UserID: userID, FormTemplateID: ToNI(other)}))
	must(t)(db.NotificationAdd(&Notification{Name: "impreza", Type: "email", RelatedTo: "events", UserID: userID}))
	nn, err := formNotifications(db, user, templ)
	if err != nil {
		t.Fatal(err)
	}
	if len(nn) != 2 || nn[0].ID != free || nn[1].ID != own {
		t.Errorf("maile formularza: %+v", nn)
	}
	claimMail(db, user, FormTemplate{ID: other}, free)
	if n, _ := db.NotificationGetByID(free, userID); n.FormTemplateID.Int64 != other {
		t.Errorf("wolny mail nie przypisany: %+v", n)
	}
	claimMail(db, user, templ, free) // już należy do innego
	if n, _ := db.NotificationGetByID(free, userID); n.FormTemplateID.Int64 != other {
		t.Errorf("przypisany mail przejęty: %+v", n)
	}
}

// Kopia formularza dostaje własne kopie maili, nazwane od nowego formularza.
func TestCopyFormMails(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	user := User{ID: userID}
	guest := must(t)(db.NotificationAdd(&Notification{Name: "Bal 2026 gość", Type: "email", RelatedTo: "forms", Text: "<p>g</p>", UserID: userID}))
	admin := must(t)(db.NotificationAdd(&Notification{Name: "Bal 2026 admin", Type: "email", RelatedTo: "forms", Text: "<p>a</p>", UserID: userID}))
	src := FormTemplate{Name: "Bal 2026", URL: "bal26", UserID: userID, CreatedDate: 1, NotificationID: ToNI(guest), AdminNotificationID: ToNI(admin)}
	src.ID = must(t)(db.FormTemplateAdd(&src))
	newID := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Bal 2027", URL: "bal27", UserID: userID, CreatedDate: 1}))

	copyFormMails(db, user, src, newID, "Bal 2027", "pl")
	nt, err := db.FormTemplateGetByID(newID, userID)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := db.NotificationGetByID(nt.NotificationID.Int64, userID)
	a, _ := db.NotificationGetByID(nt.AdminNotificationID.Int64, userID)
	if g.ID == guest || a.ID == admin || g.ID == a.ID {
		t.Fatalf("maile nie skopiowane: gość %d, admin %d", g.ID, a.ID)
	}
	if g.Name != "Bal 2027 – potwierdzenie" || a.Name != "Bal 2027 – dla administratora" || g.Text != "<p>g</p>" {
		t.Errorf("kopie: %q %q %q", g.Name, a.Name, g.Text)
	}
	if g.FormTemplateID.Int64 != newID || a.FormTemplateID.Int64 != newID {
		t.Errorf("kopie nie należą do nowego formularza: %+v %+v", g.FormTemplateID, a.FormTemplateID)
	}

	// ten sam mail dla gościa i administratora - jedna kopia
	src.AdminNotificationID = ToNI(guest)
	third := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Bal 2028", URL: "bal28", UserID: userID, CreatedDate: 1}))
	copyFormMails(db, user, src, third, "Bal 2028", "pl")
	if nt, _ := db.FormTemplateGetByID(third, userID); nt.NotificationID.Int64 == 0 || nt.NotificationID.Int64 != nt.AdminNotificationID.Int64 {
		t.Errorf("wspólny mail: %+v %+v", nt.NotificationID, nt.AdminNotificationID)
	}
}
