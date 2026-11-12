package main

// form_fill.go: menedżer dopisuje osoby do formularza - /admin/forms/{id}/fill.
//
// Pole checkbox z ustawieniem "manager dopisuje osoby" (mgr_people) to np.
// funkcja na balu: zamiast zaznaczać ją za siebie, menedżer dopisuje przy niej
// osoby (imię, nazwisko, e-mail). Każda osoba to zwykłe zgłoszenie formularza
// (forms) z zaznaczonymi funkcjami, więc raporty, liczniki i eksport działają
// bez zmian:
//   - osoba, która ma już zgłoszenie (ten sam e-mail), dostaje funkcję do
//     swojego zgłoszenia;
//   - nowa osoba dostaje nowe zgłoszenie z wartościami pozostałych pól, które
//     menedżer wypełnił (np. klasa, od której zależy, jakie funkcje widać);
//   - forms.added_by / formanswers.added_by = kto dopisał (Login.AdderID), więc
//     menedżer może usuwać tylko to, co sam dopisał; zgłoszenie, które założył,
//     znika razem z ostatnią jego funkcją.
//
// Pola z ustawieniem "niewidoczne dla menedżerów" (mgr_hidden) i pola z rolą
// imienia, nazwiska i e-maila nie pokazują się w trybie dopisywania - dane
// osoby wpisuje się przy funkcji. Wpisane osoby trafiają do manager_contacts
// i są podpowiadane w każdym formularzu (dla tego samego menedżera).
// Po zapisie nowe osoby dostają mail (formtemplates.mgr_mail, szablon
// mgr_notifications_id_fk albo ten sam, co dla gościa).

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/sessions"
)

// isPeopleField - pole, przy którym menedżer dopisuje osoby.
func isPeopleField(d FormDefField) bool {
	return d.S("type") == "checkbox-group" && d.B("mgr_people") && d.S("name") != ""
}

// peopleFields - pola z dopisywaniem osób.
func peopleFields(defs []FormDefField) []FormDefField {
	var pp []FormDefField
	for _, d := range defs {
		if isPeopleField(d) {
			pp = append(pp, d)
		}
	}
	return pp
}

// hiddenForManager - pole nie pokazuje się w trybie dopisywania.
func hiddenForManager(d FormDefField) bool {
	return d.B("mgr_hidden") || isRoleName(d.S("name"))
}

// peopleValue - wartość zapisywana, gdy osoba jest przy funkcji: pierwsza
// opcja pola (np. "Tak" przy "Zgłaszam się").
func peopleValue(d FormDefField) string {
	v := ""
	eachOption(d, func(value, _ string) {
		if v == "" {
			v = value
		}
	})
	if v == "" {
		v = "1"
	}
	return v
}

// hasValue - czy odpowiedź pola z opcjami (wartości po ", ") zawiera v.
func hasValue(answer, v string) bool {
	for _, p := range strings.Split(answer, ", ") {
		if strings.TrimSpace(p) == v {
			return true
		}
	}
	return false
}

// withoutValue - odpowiedź bez wartości v.
func withoutValue(answer, v string) string {
	var out []string
	for _, p := range strings.Split(answer, ", ") {
		if p = strings.TrimSpace(p); p != "" && p != v {
			out = append(out, p)
		}
	}
	return strings.Join(out, ", ")
}

// mgrMailSummary - podsumowanie sekcji "Mail do dopisanych osób".
func mgrMailSummary(db *DB, user User, templ FormTemplate, lang string) string {
	if !templ.MgrMail {
		return T(lang, "form.mgr_mail_off")
	}
	if templ.MgrNotificationID.Valid && templ.MgrNotificationID.Int64 > 0 {
		if name, miss := notificationName(db, user, templ.MgrNotificationID.Int64, lang); !miss {
			return name
		}
	}
	if !templ.NotificationID.Valid || templ.NotificationID.Int64 == 0 {
		return T(lang, "form.mgr_mail_same") + " – " + T(lang, "common.not_set")
	}
	return T(lang, "form.mgr_mail_same")
}

// --- strona ---------------------------------------------------------------

// fillPerson - osoba przy funkcji na stronie dopisywania.
type fillPerson struct {
	Form    int64  `json:"form"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Email   string `json:"email"`
	// Mine - zalogowany może ją usunąć (sam ją dopisał)
	Mine bool `json:"mine"`
	// By - "guest" (zgłosiła się sama), "me", "other" (inny menedżer)
	By string `json:"by"`
}

type fillState struct {
	People   map[string][]fillPerson `json:"people"`
	Contacts []Contact               `json:"contacts"`
}

// canRemove - zalogowany może zdjąć osobę z funkcji: tylko to, co sam
// dopisał; konto organizacji - wszystko, co dopisali menedżerowie.
func canRemove(login Login, addedBy int64, valid bool) bool {
	if !valid {
		return false
	}
	return addedBy == login.AdderID() || !login.Manager
}

// fillPeople - osoby przy każdym polu z dopisywaniem.
func fillPeople(db *DB, login Login, templ FormTemplate, people []FormDefField) (map[string][]fillPerson, error) {
	out := map[string][]fillPerson{}
	want := map[string]string{}
	for _, d := range people {
		out[d.S("name")] = []fillPerson{}
		want[d.S("name")] = peopleValue(d)
	}
	aa, err := db.PersonAnswersForTemplate(templ.ID)
	if err != nil {
		return out, err
	}
	for _, a := range aa {
		v, ok := want[a.Field]
		if !ok || !hasValue(a.Value, v) {
			continue
		}
		p := fillPerson{Form: a.FormID, Name: a.Name.String, Surname: a.Surname.String, Email: a.Email.String, By: "guest"}
		if a.AddedBy.Valid {
			p.By = "other"
			if a.AddedBy.Int64 == login.AdderID() {
				p.By = "me"
			}
		}
		p.Mine = canRemove(login, a.AddedBy.Int64, a.AddedBy.Valid)
		out[a.Field] = append(out[a.Field], p)
	}
	return out, nil
}

type nFillVars struct {
	NPage
	ID       int64
	Name     string
	Rows     []RenderRow
	State    template.JS
	DraftKey string
	// NoPeople - formularz nie ma pól z dopisywaniem osób
	NoPeople bool
	Closed   bool
}

// FormFillPage - /admin/forms/{id}/fill
func FormFillPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, true)
		if !ok {
			return
		}
		lang := PanelLang(r)
		login := requestLogin(r)
		defs, err := ParseFormDef(templ.Content.String)
		if err != nil {
			log.Printf("FormFillPage: form %d: %v", templ.ID, err)
		}

		v := nFillVars{
			NPage: NPage{
				Title:   T(lang, "fill.title") + " – " + templ.Name,
				Crumb:   T(lang, "fill.title"),
				OrgName: orgName(user),
				UpURL:   fmt.Sprintf("/admin/forms/%d", templ.ID),
				UpLabel: templ.Name,
				NoOrg:   noOrg(r),
			},
			ID:       templ.ID,
			Name:     templ.Name,
			DraftKey: fmt.Sprintf("rezerwo.fill.%d", templ.ID),
			Closed:   !templ.IsOpen(nowUnix()),
		}
		people := peopleFields(defs)
		if len(people) == 0 {
			v.NoPeople = true
			nRender(w, r, lang, v, "tmpl/n_formfill.html", "tmpl/n_formfield.html")
			return
		}

		var shown []FormDefField
		for _, d := range defs {
			if isInputType(d.S("type")) && hiddenForManager(d) {
				continue
			}
			shown = append(shown, d)
		}
		v.Rows = BuildRenderRows(shown, "")
		isPeople := map[string]bool{}
		for _, d := range people {
			isPeople[d.S("name")] = true
		}
		goals := map[string]int64{}
		for _, d := range people {
			goals[d.S("name")] = intOf(d["panel_goal"])
		}
		for i := range v.Rows {
			for j := range v.Rows[i].Fields {
				f := &v.Rows[i].Fields[j]
				if isPeople[f.Name] {
					f.People = true
					f.Goal = goals[f.Name]
					f.Required = false
				}
			}
		}

		st := fillState{}
		if st.People, err = fillPeople(db, login, templ, people); err != nil {
			log.Printf("FormFillPage: people: %v", err)
		}
		if st.Contacts, err = db.ContactGetAll(user.ID, login.AdderID()); err != nil {
			log.Printf("FormFillPage: contacts: %v", err)
		}
		b, _ := json.Marshal(st)
		v.State = template.JS(b)
		nRender(w, r, lang, v, "tmpl/n_formfill.html", "tmpl/n_formfield.html")
	}
}

// FormFillPeopleAPI - GET /api/forms/{id}/fill/people: aktualne osoby przy
// funkcjach; strona dopisywania odpytuje co 30 s (RZ.live), żeby widzieć
// zgłoszenia gości i innych menedżerów bez przeładowania.
func FormFillPeopleAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		defs, err := ParseFormDef(templ.Content.String)
		if err != nil {
			log.Printf("FormFillPeopleAPI: form %d: %v", templ.ID, err)
		}
		people, err := fillPeople(db, requestLogin(r), templ, peopleFields(defs))
		if err != nil {
			log.Printf("FormFillPeopleAPI: people: %v", err)
			jsonMsg(w, http.StatusInternalServerError, map[string]string{"msg": "error"})
			return
		}
		writeLive(w, r, map[string]interface{}{"people": people})
	}
}

// --- zapis ----------------------------------------------------------------

type fillAdd struct {
	Field   string `json:"field"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Email   string `json:"email"`
}

type fillRemove struct {
	Field string `json:"field"`
	Form  int64  `json:"form"`
}

type fillMsg struct {
	// Data - wartości pozostałych pól (np. klasa) - trafiają do nowych zgłoszeń
	Data   []FormAnswersJson `json:"data"`
	Add    []fillAdd         `json:"add"`
	Remove []fillRemove      `json:"remove"`
}

type fillResp struct {
	Msg    string                  `json:"msg"`
	People map[string][]fillPerson `json:"people"`
	// Contacts - podpowiedzi po zapisie (z nowymi osobami)
	Contacts []Contact `json:"contacts"`
	Added    int       `json:"added"`
	Removed  int       `json:"removed"`
	Mails    int       `json:"mails"`
}

// cleanPerson sprawdza dane dopisywanej osoby; zwraca komunikat błędu.
func cleanPerson(lang string, a *fillAdd) string {
	a.Name = strings.Join(strings.Fields(a.Name), " ")
	a.Surname = strings.Join(strings.Fields(a.Surname), " ")
	a.Email = strings.ToLower(strings.TrimSpace(a.Email))
	if a.Name == "" || a.Surname == "" {
		return T(lang, "fill.err.name")
	}
	if utf8.RuneCountInString(a.Name) > 100 || utf8.RuneCountInString(a.Surname) > 100 {
		return T(lang, "fill.err.name")
	}
	if !validEmail(a.Email) {
		return T(lang, "fill.err.email", a.Email)
	}
	return ""
}

// sharedDefs - pola wspólne trybu dopisywania (bez pól z osobami i pól
// niewidocznych dla menedżera) razem z ich widocznością.
func sharedDefs(defs []FormDefField, vis []bool) ([]FormDefField, []bool) {
	var dd []FormDefField
	var vv []bool
	for i, d := range defs {
		if isInputType(d.S("type")) && (hiddenForManager(d) || isPeopleField(d)) {
			continue
		}
		dd = append(dd, d)
		vv = append(vv, vis[i])
	}
	return dd, vv
}

// FormFillAPI - POST /api/forms/{id}/fill
func FormFillAPI(db *DB, mailConf MailConfig, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		lang := PanelLang(r)
		login := requestLogin(r)
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		var msg fillMsg
		if err := json.Unmarshal(body, &msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		defs, err := ParseFormDef(templ.Content.String)
		if err != nil {
			log.Printf("FormFillAPI: form %d: %v", templ.ID, err)
		}
		people := map[string]FormDefField{}
		for _, d := range peopleFields(defs) {
			people[d.S("name")] = d
		}

		// tylko pola wspólne; osoby i pola ukryte przed menedżerem nie przychodzą
		var data []FormAnswersJson
		for _, d := range msg.Data {
			for _, f := range defs {
				if f.S("name") == d.Name && !hiddenForManager(f) && !isPeopleField(f) {
					data = append(data, d)
					break
				}
			}
		}
		vis := ApplyConds(defs, data)
		visible := map[string]bool{}
		for i, d := range defs {
			if vis[i] {
				visible[d.S("name")] = true
			}
		}

		if len(msg.Add) > 0 {
			sd, sv := sharedDefs(defs, vis)
			if bad := checkRequired(sd, sv, data); bad != "" {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "gform.err.missing", bad), "field": bad})
				return
			}
			if bad, why := checkValues(sd, sv, data, "", lang); bad != "" {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": bad + ": " + why, "field": bad})
				return
			}
		}
		for i := range msg.Add {
			a := &msg.Add[i]
			if _, ok := people[a.Field]; !ok || !visible[a.Field] {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "fill.err.field")})
				return
			}
			if bad := cleanPerson(lang, a); bad != "" {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": bad, "field": a.Field})
				return
			}
		}

		fields, err := db.FormFieldGetAllForTmpl(templ.ID)
		if err != nil {
			log.Printf("FormFillAPI: fields of %d: %v", templ.ID, err)
		}
		fieldIDs := map[string]int64{}
		for _, f := range fields {
			if _, dup := fieldIDs[f.Name]; !dup {
				fieldIDs[f.Name] = f.ID
			}
		}
		adder := ToNI(login.AdderID())
		if login.AdderID() == 0 {
			adder.Valid = true // konto organizacji: 0, nie NULL (NULL = gość)
		}
		now := nowUnix()
		resp := fillResp{Msg: "saved"}

		// zdejmowanie z funkcji - tylko własne wpisy
		for _, rm := range msg.Remove {
			d, ok := people[rm.Field]
			fid := fieldIDs[rm.Field]
			if !ok || fid == 0 {
				continue
			}
			f, err := db.FormGet(rm.Form, templ.ID)
			if err != nil {
				continue
			}
			ans, err := db.FormAnswerGet(f.ID, fid)
			if err != nil || !canRemove(login, ans.AddedBy.Int64, ans.AddedBy.Valid) {
				jsonMsg(w, http.StatusForbidden, map[string]string{"msg": T(lang, "fill.err.not_mine")})
				return
			}
			rest := withoutValue(ans.Value.String, peopleValue(d))
			if rest == "" {
				err = db.FormAnswerDelOne(f.ID, fid)
			} else {
				ans.Value = ToNS(rest)
				err = db.FormAnswerSet(&ans)
			}
			if err != nil {
				log.Printf("FormFillAPI: remove %s from %d: %v", rm.Field, f.ID, err)
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error")})
				return
			}
			resp.Removed++
			// zgłoszenie założone przez tego samego dopisującego znika razem
			// z ostatnią funkcją
			if f.AddedBy.Valid && canRemove(login, f.AddedBy.Int64, true) && !hasPeopleAnswer(db, f.ID, fieldIDs, people) {
				if err := db.FormDelWithAnswers(f.ID, templ.ID); err != nil {
					log.Printf("FormFillAPI: delete form %d: %v", f.ID, err)
				}
			}
		}

		// dopisywanie: jedna osoba (e-mail) = jedno zgłoszenie
		byEmail := map[string][]fillAdd{}
		var order []string
		for _, a := range msg.Add {
			if _, ok := byEmail[a.Email]; !ok {
				order = append(order, a.Email)
			}
			byEmail[a.Email] = append(byEmail[a.Email], a)
		}
		type mailTo struct {
			formID        int64
			name, surname string
		}
		var mails []mailTo
		for _, email := range order {
			adds := byEmail[email]
			p := adds[0]
			formID, created, err := personForm(db, user, templ, p, adder, now, data, vis, defs, fieldIDs)
			if err != nil {
				log.Printf("FormFillAPI: person %q: %v", email, err)
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error")})
				return
			}
			gotNew := created
			for _, a := range adds {
				d := people[a.Field]
				fid := fieldIDs[a.Field]
				if fid == 0 {
					continue
				}
				val := peopleValue(d)
				ans, err := db.FormAnswerGet(formID, fid)
				if err == nil && hasValue(ans.Value.String, val) {
					continue // już jest przy tej funkcji
				}
				nv := val
				if err == nil && strings.TrimSpace(ans.Value.String) != "" {
					nv = ans.Value.String + ", " + val
				}
				if err := db.FormAnswerSet(&FormAnswer{Value: ToNS(nv), FormFieldID: fid, FormID: formID, AddedBy: adder}); err != nil {
					log.Printf("FormFillAPI: answer %s of %d: %v", a.Field, formID, err)
					jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error")})
					return
				}
				resp.Added++
				gotNew = true
			}
			if err := db.ContactSave(user.ID, login.AdderID(), Contact{Name: p.Name, Surname: p.Surname, Email: p.Email}, now); err != nil {
				log.Printf("FormFillAPI: contact %q: %v", email, err)
			}
			if gotNew {
				mails = append(mails, mailTo{formID, p.Name, p.Surname})
			}
		}
		log.Printf("FormFillAPI: %q form %d: +%d -%d", login.Email, templ.ID, resp.Added, resp.Removed)

		if templ.MgrMail && len(mails) > 0 {
			mailID := templ.NotificationID
			if templ.MgrNotificationID.Valid && templ.MgrNotificationID.Int64 > 0 {
				mailID = templ.MgrNotificationID
			}
			if mailID.Valid && mailID.Int64 > 0 {
				resp.Mails = len(mails)
				go func() {
					for _, m := range mails {
						sendFillMail(db, mailConf, user, templ, mailID.Int64, m.formID, m.name, m.surname)
					}
				}()
			}
		}

		pp := peopleFields(defs)
		if resp.People, err = fillPeople(db, login, templ, pp); err != nil {
			log.Printf("FormFillAPI: people: %v", err)
		}
		if resp.Contacts, err = db.ContactGetAll(user.ID, login.AdderID()); err != nil {
			log.Printf("FormFillAPI: contacts: %v", err)
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

// hasPeopleAnswer - zgłoszenie jest jeszcze przy jakiejś funkcji.
func hasPeopleAnswer(db *DB, formID int64, fieldIDs map[string]int64, people map[string]FormDefField) bool {
	aa, err := db.FormAnswerGetAll(formID)
	if err != nil {
		return true
	}
	ids := map[int64]string{}
	for name, d := range people {
		ids[fieldIDs[name]] = peopleValue(d)
	}
	for _, a := range aa {
		if v, ok := ids[a.FormFieldID]; ok && hasValue(a.Value.String, v) {
			return true
		}
	}
	return false
}

// personForm - zgłoszenie osoby: istniejące (ten sam e-mail) albo nowe
// z wartościami pól wspólnych i rolami imienia, nazwiska i e-maila.
func personForm(db *DB, user User, templ FormTemplate, p fillAdd, adder sql.NullInt64, now int64,
	data []FormAnswersJson, vis []bool, defs []FormDefField, fieldIDs map[string]int64) (int64, bool, error) {
	ff, err := db.FormGetAllByEmail(p.Email, templ.ID, user.ID)
	if err != nil {
		return 0, false, err
	}
	if len(ff) > 0 {
		return ff[0].ID, false, nil
	}
	f := &Form{
		Name:           ToNS(p.Name),
		Surname:        ToNS(p.Surname),
		Email:          ToNS(p.Email),
		CreatedDate:    now,
		UserID:         user.ID,
		FormTemplateID: templ.ID,
		AddedBy:        adder,
	}
	formID, err := db.FormAdd(f)
	if err != nil {
		return 0, false, err
	}
	shown := map[string]bool{}
	for i, d := range defs {
		shown[d.S("name")] = vis[i]
	}
	set := func(name, value string) {
		id := fieldIDs[name]
		if id == 0 || value == "" {
			return
		}
		if err := db.FormAnswerSet(&FormAnswer{Value: ToNS(value), FormFieldID: id, FormID: formID, AddedBy: adder}); err != nil {
			log.Printf("personForm: answer %s of %d: %v", name, formID, err)
		}
	}
	set(ROLENAME, p.Name)
	set(ROLESURNAME, p.Surname)
	set(ROLEEMAIL, p.Email)
	for _, d := range data {
		if shown[d.Name] {
			set(d.Name, strings.TrimSpace(strings.Join(d.UserData, ", ")))
		}
	}
	return formID, true, nil
}

// sendFillMail - mail do osoby dopisanej przez menedżera (bez linku do
// poprawiania - zgłoszenia nie wypełniała sama).
func sendFillMail(db *DB, mailConf MailConfig, user User, templ FormTemplate, mailID, formID int64, name, surname string) {
	form, err := db.FormGet(formID, templ.ID)
	if err != nil || form.Email.String == "" {
		return
	}
	mail, err := db.NotificationGetByID(mailID, user.ID)
	if err != nil {
		log.Printf("sendFillMail: mail %d: %v", mailID, err)
		return
	}
	text, imgs := parseFormMailTemplate(name, surname, mail.Text, formID, user, templ, db)
	subject := formMailSubject(name, surname, mail.Title.String, formID, user, templ, db)
	if err := mailConf.SendAsOrg(user, form.Email.String, subject, text, mail.Files(user), append(mail.Images(user), imgs...)); err != nil {
		log.Printf("sendFillMail: %v", err)
	}
}

// FormContactDelAPI - DELETE /api/forms/{id}/contacts?email=... - usuwa
// podpowiedź zalogowanego (lista jest wspólna dla wszystkich formularzy).
func FormContactDelAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		login := requestLogin(r)
		if err := db.ContactDel(user.ID, login.AdderID(), r.URL.Query().Get("email")); err != nil {
			log.Printf("FormContactDelAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(PanelLang(r), "common.save_error")})
			return
		}
		cc, _ := db.ContactGetAll(user.ID, login.AdderID())
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "deleted", "contacts": cc})
	}
}
