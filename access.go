package main

// access.go: uprawnienia menedżerów (tools/migration-managerperms.sql).
//
// Konto organizacji może wszystko. Menedżer ma dwa przełączniki (admins):
//   - all_access: pełny dostęp do wszystkich imprez i formularzy; bez niego
//     tylko do wybranych w manager_access, każdy z poziomem:
//     LEVELRESULTS ("obsługa": rezerwacje, odpowiedzi, raporty, maile do gości,
//     dopisywanie osób - wszystko poza zmianą ustawień) albo LEVELFULL;
//   - org_manage: zarządzanie organizacją - katalog (sale, maile, konta),
//     projektant sal, nowe imprezy i formularze, kopiowanie imprez.
//
// Sprawdzamy w jednym miejscu: adminUser (admin_ui.go) przepuszcza żądanie
// tylko wtedy, gdy routeNeed dla jego adresu jest spełnione. Adres, którego
// nie ma w accessRules, wymaga org_manage - nowa trasa panelu jest więc
// domyślnie zamknięta dla menedżera z ograniczeniami. Listy (strona startowa)
// filtrują pozycje przez Login.Level.

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	ACCEVENT = "event"
	ACCFORM  = "form"

	LEVELRESULTS = "results"
	LEVELFULL    = "full"
)

// levelRank - porównanie poziomów: brak < obsługa < pełny.
func levelRank(l string) int {
	switch l {
	case LEVELFULL:
		return 2
	case LEVELRESULTS:
		return 1
	}
	return 0
}

// validLevel - poziom zapisany w manager_access.
func validLevel(l string) bool { return l == LEVELRESULTS || l == LEVELFULL }

func accessKey(kind string, id int64) string { return kind + ":" + strconv.FormatInt(id, 10) }

// AdderID - identyfikator zalogowanego w forms.added_by / formanswers.added_by
// i manager_contacts: admins.id menedżera, 0 dla konta organizacji.
func (l Login) AdderID() int64 {
	if l.Manager {
		return l.AdminID
	}
	return 0
}

// CanOrg - może zarządzać organizacją (katalog, nowe imprezy i formularze).
func (l Login) CanOrg() bool { return !l.Manager || l.OrgManage }

// Restricted - menedżer widzi tylko wybrane imprezy i formularze.
func (l Login) Restricted() bool { return l.Manager && !l.AllAccess }

// Level - poziom dostępu do imprezy albo formularza: "", LEVELRESULTS,
// LEVELFULL.
func (l Login) Level(kind string, id int64) string {
	if !l.Restricted() {
		return LEVELFULL
	}
	return l.Access[accessKey(kind, id)]
}

// CanEdit - może zmieniać ustawienia imprezy/formularza.
func (l Login) CanEdit(kind string, id int64) bool { return l.Level(kind, id) == LEVELFULL }

// anyFull - ma pełny dostęp do czegokolwiek (np. wgrywanie bannerów).
func (l Login) anyFull() bool {
	if !l.Restricted() {
		return true
	}
	for _, v := range l.Access {
		if v == LEVELFULL {
			return true
		}
	}
	return false
}

const (
	needAny     = "any"     // każdy zalogowany
	needOrg     = "org"     // org_manage
	needAnyFull = "anyfull" // pełny dostęp do czegokolwiek
	needResults = LEVELRESULTS
	needFull    = LEVELFULL
)

type accessRule struct {
	re     *regexp.Regexp
	method string // "" = każda metoda
	kind   string // ACCEVENT / ACCFORM: id z pierwszej grupy wyrażenia
	need   string
}

func rule(re, method, kind, need string) accessRule {
	return accessRule{regexp.MustCompile(re), method, kind, need}
}

// accessRules - pierwsza pasująca reguła decyduje. Bez dopasowania: needOrg.
var accessRules = []accessRule{
	rule(`^/admin/?$`, "", "", needAny),
	rule(`^/admin/start$`, "", "", needAny),
	rule(`^/admin/managers$`, "", "", needAny),

	rule(`^/admin/events/(\d+)$`, "", ACCEVENT, needResults),
	rule(`^/admin/events/(\d+)/reservations$`, "", ACCEVENT, needResults),
	rule(`^/api/events/(\d+)$`, "", ACCEVENT, needFull),
	rule(`^/api/events/(\d+)/section$`, "", ACCEVENT, needFull),
	rule(`^/api/events/(\d+)/addons$`, "", ACCEVENT, needFull),
	rule(`^/api/events/(\d+)/(reservations|reservations\.csv|report\.csv|addons\.csv|reservations/status|reservations/history|reservations/notes|reservations/mail)$`, "", ACCEVENT, needResults),
	rule(`^/api/events/(\d+)/orders/\d+/items(/\d+(/status)?)?$`, "", ACCEVENT, needResults),

	rule(`^/admin/forms/(\d+)$`, "", ACCFORM, needResults),
	rule(`^/admin/forms/(\d+)/answers$`, "", ACCFORM, needResults),
	rule(`^/admin/forms/(\d+)/fill$`, "", ACCFORM, needResults),
	rule(`^/admin/forms/(\d+)/fields$`, "", ACCFORM, needFull),
	rule(`^/api/forms/(\d+)/(fields|section|maildraft)$`, "", ACCFORM, needFull),
	rule(`^/api/forms/(\d+)$`, "", ACCFORM, needFull),
	rule(`^/api/forms/(\d+)/(answers|answers\.csv|answers/status|answers/mail|infopanel|fill|contacts)$`, "", ACCFORM, needResults),

	rule(`^/api/media/banners$`, "", "", needAnyFull),
	// zapamiętane osoby: menedżer tylko swoje (sprawdza adminContact)
	rule(`^/api/contacts/\d+$`, "", "", needAny),
}

// routeNeed - czego wymaga adres: need i (dla imprezy/formularza) jego id.
func routeNeed(r *http.Request) (need, kind string, id int64) {
	p := r.URL.Path
	for _, ru := range accessRules {
		if ru.method != "" && ru.method != r.Method {
			continue
		}
		m := ru.re.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		if ru.kind != "" {
			id, _ = strconv.ParseInt(m[1], 10, 64)
		}
		return ru.need, ru.kind, id
	}
	return needOrg, "", 0
}

// allowed - czy zalogowany może wykonać żądanie r. Dodatki imprezy to
// formularz (order_addons) - liczy się dostęp do jego imprezy.
func allowed(db *DB, login Login, r *http.Request) bool {
	if !login.Manager || (login.AllAccess && login.OrgManage) {
		return true
	}
	need, kind, id := routeNeed(r)
	switch need {
	case needAny:
		return true
	case needOrg:
		return login.CanOrg()
	case needAnyFull:
		return login.anyFull()
	}
	if kind == ACCFORM && login.Restricted() {
		if t, err := db.FormTemplateGetByID(id, login.User.ID); err == nil && isAddonsTemplate(t) && t.EventID.Valid {
			kind, id = ACCEVENT, t.EventID.Int64
		}
	}
	return levelRank(login.Level(kind, id)) >= levelRank(need)
}

// denyAccess - odpowiedź na żądanie bez uprawnień: JSON dla /api, strona
// błędu dla reszty.
func denyAccess(w http.ResponseWriter, r *http.Request, login Login) {
	lang := PanelLang(r)
	log.Printf("access: %q denied %s %s", login.Email, r.Method, r.URL.Path)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		jsonMsg(w, http.StatusForbidden, map[string]string{"msg": T(lang, "perm.denied")})
		return
	}
	w.WriteHeader(http.StatusForbidden)
	nRender(w, lang, nPanelErrorVars{
		NPage: NPage{Title: T(lang, "perm.denied_title"), OrgName: orgName(login.User), NoOrg: !login.CanOrg()},
		Text:  T(lang, "perm.denied"),
	}, "tmpl/n_panel_error.html")
}

// nPanelErrorVars - strona błędu w panelu (tmpl/n_panel_error.html).
type nPanelErrorVars struct {
	NPage
	Text string
}

// loadAccess uzupełnia Login menedżera o uprawnienia z bazy.
func loadAccess(db *DB, l *Login, a Admin) {
	l.AdminID = a.ID
	l.AllAccess = a.AllAccess
	l.OrgManage = a.OrgManage
	if a.AllAccess {
		return
	}
	l.Access = map[string]string{}
	aa, err := db.ManagerAccessGet(a.ID)
	if err != nil {
		log.Printf("loadAccess %d: %v", a.ID, err)
		return
	}
	for _, x := range aa {
		l.Access[accessKey(x.Kind, x.ItemID)] = x.Level
	}
}
