package main

// orderform.go: formularz zamówienia miejsc - cały edytowalny w designerze pól.
//
// Formularz to szablon rodzaju order_addons przypięty do imprezy
// (events.addons_formtemplate_id), ten sam, w którym wcześniej były same
// dodatki. Dane osoby (e-mail, imię, nazwisko, telefon, uwagi) są w nim
// polami systemowymi o stałych nazwach (ORDERSYS): organizator zmienia im
// etykietę, opis, szerokość i miejsce, ale ich wartości nie idą do
// formanswers, tylko jak dotąd do customers / notes. Pozostałe pola to
// dodatki (addons.go), do tego nagłówki, akapity i oddzielacze.
//
// Imprezy sprzed tej zmiany (bez szablonu albo z szablonem bez pól
// systemowych) dostają formularz złożony w locie z dawnego układu:
// stałe dane osoby i dodatki w miejscu z events.addons_after
// (LegacyOrderDef). Zapisuje się on dopiero przy pierwszym zapisie
// w designerze, więc wdrożenie nie wymaga migracji.
//
// Pola systemowe idą w formularzu gościa pod dawnymi nazwami (email, name,
// surname, phone, notes), więc /order/status czyta je bez zmian - także
// z formularza wczytanego przed wdrożeniem.

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Role pól tylko w formularzu zamówienia (e-mail, imię i nazwisko mają role
// ze zwykłych formularzy: ROLEEMAIL, ROLENAME, ROLESURNAME).
const (
	ROLEPHONE = "phone-ROLE"
	ROLENOTES = "notes-ROLE"
)

// orderSysField opisuje pole systemowe formularza zamówienia.
type orderSysField struct {
	Name  string // nazwa pola w definicji
	Input string // nazwa pola w formularzu gościa (/order/status)
	// Locked - pola nie da się usunąć i zawsze jest obowiązkowe
	Locked bool
}

// ORDERSYS - pola systemowe w kolejności domyślnego formularza.
var ORDERSYS = []orderSysField{
	{ROLEEMAIL, "email", true},
	{ROLENAME, "name", true},
	{ROLESURNAME, "surname", true},
	{ROLEPHONE, "phone", false},
	{ROLENOTES, "notes", false},
}

// orderSys zwraca opis pola systemowego o tej nazwie.
func orderSys(name string) (orderSysField, bool) {
	for _, s := range ORDERSYS {
		if s.Name == name {
			return s, true
		}
	}
	return orderSysField{}, false
}

// isOrderSys mówi, czy pole definicji jest polem systemowym zamówienia.
func isOrderSys(name string) bool {
	_, ok := orderSys(name)
	return ok
}

// hasOrderSys mówi, czy definicja jest już formularzem zamówienia (ma choć
// jedno pole systemowe), a nie dawną listą samych dodatków.
func hasOrderSys(defs []FormDefField) bool {
	for _, d := range defs {
		if isOrderSys(d.S("name")) {
			return true
		}
	}
	return false
}

// defaultSysField - pole systemowe z domyślną etykietą w języku imprezy,
// tak jak wyglądało w dawnym stałym formularzu.
func defaultSysField(name, lang, notesHelp string) FormDefField {
	f := FormDefField{"name": name, "access": false, "className": "form-control"}
	switch name {
	case ROLEEMAIL:
		f["type"], f["subtype"] = "text", "email"
		f["label"] = T(lang, "order.email")
		f["description"] = T(lang, "order.email_help")
		f["required"] = true
	case ROLENAME:
		f["type"], f["subtype"] = "text", "text"
		f["label"] = T(lang, "order.name")
		f["required"] = true
		f["className"] = "form-control row-sysname col-md-6"
	case ROLESURNAME:
		f["type"], f["subtype"] = "text", "text"
		f["label"] = T(lang, "order.surname")
		f["required"] = true
		f["className"] = "form-control row-sysname col-md-6"
	case ROLEPHONE:
		f["type"], f["subtype"] = "text", "tel"
		f["label"] = T(lang, "order.phone")
		f["placeholder"] = T(lang, "order.phone_placeholder")
		f["required"] = false
		f["className"] = "form-control row-sysphone col-md-6"
	case ROLENOTES:
		f["type"], f["subtype"] = "textarea", "textarea"
		f["label"] = T(lang, "order.notes")
		f["rows"] = "3"
		f["required"] = false
		if notesHelp = strings.TrimSpace(notesHelp); notesHelp != "" {
			f["description"] = notesHelp
		}
	}
	return f
}

// LegacyOrderDef składa formularz zamówienia z dawnego układu: stałe dane
// osoby, a dodatki (addons - dawna definicja, bez pól systemowych) w miejscu
// z events.addons_after; przy events.addons_section z nagłówkiem
// "Dodatki do zamówienia" zamiast dawnej ramki.
func LegacyOrderDef(addons []FormDefField, after string, section bool, lang, notesHelp string) []FormDefField {
	var block []FormDefField
	for _, d := range addons {
		if !isOrderSys(d.S("name")) {
			block = append(block, d)
		}
	}
	if section && len(block) > 0 {
		head := FormDefField{"type": "header", "subtype": "h4", "label": T(lang, "addons.guest_title"), "access": false}
		block = append([]FormDefField{head}, block...)
	}

	sys := func(name string) FormDefField { return defaultSysField(name, lang, notesHelp) }
	var out []FormDefField
	after = AddonsAfterNorm(after)
	if after == "start" {
		out = append(out, block...)
	}
	out = append(out, sys(ROLEEMAIL))
	if after == "email" {
		out = append(out, block...)
	}
	out = append(out, sys(ROLENAME), sys(ROLESURNAME))
	if after == "name" {
		out = append(out, block...)
	}
	out = append(out, sys(ROLEPHONE))
	if after == "phone" {
		out = append(out, block...)
	}
	out = append(out, sys(ROLENOTES))
	if after == "" {
		out = append(out, block...)
	}
	return out
}

// NormalizeOrderDef przygotowuje definicję formularza zamówienia: dawną
// listę dodatków zamienia na pełny formularz, brakujące pola obowiązkowe
// (e-mail, imię, nazwisko) dokłada na początek, a polom systemowym
// przywraca ich rodzaj - designer na to nie pozwala, ale stara kopia robocza
// albo ręcznie wysłany JSON mógłby.
func NormalizeOrderDef(defs []FormDefField, event Event) []FormDefField {
	lang := KnownLang(event.Language.String)
	if !hasOrderSys(defs) {
		return LegacyOrderDef(defs, event.AddonsAfter, event.AddonsSection, lang, event.OrderNotesDescription)
	}
	have := map[string]bool{}
	for _, d := range defs {
		if n := d.S("name"); isOrderSys(n) {
			have[n] = true
			fixSysField(d)
		}
	}
	var missing []FormDefField
	for _, s := range ORDERSYS {
		if s.Locked && !have[s.Name] {
			missing = append(missing, defaultSysField(s.Name, lang, event.OrderNotesDescription))
		}
	}
	if len(missing) == 0 {
		return defs
	}
	return append(missing, defs...)
}

// fixSysField - rodzaj pola systemowego jest stały: e-mail, imię i nazwisko
// to pola tekstowe (obowiązkowe), telefon pole tel, uwagi pole wielowierszowe.
func fixSysField(d FormDefField) {
	switch d.S("name") {
	case ROLEEMAIL:
		d["type"], d["subtype"], d["required"] = "text", "email", true
	case ROLENAME, ROLESURNAME:
		d["type"], d["subtype"], d["required"] = "text", "text", true
	case ROLEPHONE:
		d["type"], d["subtype"] = "text", "tel"
	case ROLENOTES:
		d["type"], d["subtype"] = "textarea", "textarea"
	}
	for _, k := range []string{"show_if", "depends_on", "multi_empty", "multi_btn"} {
		delete(d, k)
	}
}

// OrderFormDef zwraca formularz zamówienia imprezy: zapisany w szablonie
// albo złożony z dawnego układu (LegacyOrderDef).
func OrderFormDef(db *DB, event Event) []FormDefField {
	var defs []FormDefField
	if event.AddonsFormTemplateID.Valid && event.AddonsFormTemplateID.Int64 != 0 {
		templ, err := db.FormTemplateGetByID(event.AddonsFormTemplateID.Int64, event.UserID)
		if err != nil {
			log.Printf("OrderFormDef: template %d: %v", event.AddonsFormTemplateID.Int64, err)
		} else if defs, err = ParseFormDef(templ.Content.String); err != nil {
			log.Printf("OrderFormDef: template %d: %v", templ.ID, err)
			defs = nil
		}
	}
	return NormalizeOrderDef(defs, event)
}

// --- strona gościa ---------------------------------------------------------

// nOrderItem to jedna komórka wiersza formularza zamówienia: pole danych
// osoby, nagłówek / akapit / oddzielacz (Field, szablon "ffield") albo pole
// dodatków (Addon, szablon "orderaddon").
type nOrderItem struct {
	Field *RenderField
	Addon *nOrderAddon
}

// nOrderRow - wiersz pól, jak .ff-row formularzy gościa.
type nOrderRow struct {
	Items []nOrderItem
}

// orderFormRows układa formularz zamówienia w wiersze dla strony gościa
// i zwraca też płaską listę pól dodatków (do podsumowania dopłat). Pola
// powtarzane wg liczby miejsc stają w wierszu swojego pola, jedno po drugim.
func orderFormRows(defs []FormDefField, seats int64) ([]nOrderRow, []nOrderAddon) {
	content, err := json.Marshal(defs)
	if err != nil {
		log.Printf("orderFormRows: %v", err)
	}
	addons := map[string]AddonField{}
	for _, f := range ParseAddonFields(string(content)) {
		addons[f.Name] = f
	}

	var rows []nOrderRow
	var flat []nOrderAddon
	for _, r := range BuildRenderRows(defs, "") {
		var row nOrderRow
		for _, f := range r.Fields {
			f := f
			f.ShowIf, f.DependsOn = "", "" // warunków formularz zamówienia nie ma
			if s, ok := orderSys(f.Name); ok {
				f.Name, f.ID = s.Input, s.Input
				if s.Locked {
					f.Required = true
				}
				row.Items = append(row.Items, nOrderItem{Field: &f})
				continue
			}
			if !f.IsInput() {
				row.Items = append(row.Items, nOrderItem{Field: &f})
				continue
			}
			af, ok := addons[f.Name]
			if !ok {
				continue
			}
			for _, a := range orderAddonFields([]AddonField{af}, seats) {
				a := a
				a.Width = f.Width
				row.Items = append(row.Items, nOrderItem{Addon: &a})
				flat = append(flat, a)
			}
		}
		if len(row.Items) > 0 {
			rows = append(rows, row)
		}
	}
	return rows, flat
}

// --- panel -----------------------------------------------------------------

// newOrderTemplate zakłada imprezie szablon formularza zamówienia z podaną
// definicją i przypina go do niej.
func newOrderTemplate(db *DB, user User, event Event, name string, defs []FormDefField) (int64, error) {
	id, err := db.FormTemplateAdd(&FormTemplate{
		Name: name,
		// adres nie jest używany (formularz nie ma własnej strony), ale musi
		// być unikatowy w obrębie organizacji
		URL:         fmt.Sprintf("dodatki-%d", event.ID),
		CreatedDate: time.Now().Unix(),
		UserID:      user.ID,
		EventID:     ToNI(event.ID),
	})
	if err != nil {
		return 0, err
	}
	if err := db.FormTemplateSetKind(id, user.ID, ADDONSKIND); err != nil {
		log.Printf("newOrderTemplate: set kind: %v", err)
	}
	if err := saveOrderDef(db, user, id, defs); err != nil {
		return 0, err
	}
	if err := db.EventSetAddonsTemplate(event.ID, user.ID, ToNI(id)); err != nil {
		return 0, err
	}
	return id, nil
}

// saveOrderDef zapisuje definicję szablonu i listę pól (formfields).
func saveOrderDef(db *DB, user User, templID int64, defs []FormDefField) error {
	content, err := json.Marshal(defs)
	if err != nil {
		return err
	}
	if err := db.FormTemplateSetContent(templID, user.ID, string(content), ToNS("")); err != nil {
		return err
	}
	var ffd []FormFieldDef
	if err := json.Unmarshal(content, &ffd); err == nil {
		updateFormFieldsInDB(templID, ffd, db)
	}
	return nil
}

// nOrderFieldSum - pole formularza zamówienia w podsumowaniu sekcji imprezy.
type nOrderFieldSum struct {
	Label string
	Kind  string
	Sys   bool // dane osoby
	Req   bool
}

// orderFieldSummaries opisuje pola formularza zamówienia (bez nagłówków
// i akapitów) do sekcji imprezy.
func orderFieldSummaries(defs []FormDefField, lang string) []nOrderFieldSum {
	var out []nOrderFieldSum
	for _, d := range defs {
		t := d.S("type")
		if !isInputType(t) {
			continue
		}
		s := nOrderFieldSum{Label: plainLabel(d.S("label")), Req: d.B("required")}
		if sys, ok := orderSys(d.S("name")); ok {
			s.Sys = true
			s.Req = s.Req || sys.Locked
			s.Kind = T(lang, "fields.role."+strings.TrimSuffix(strings.TrimSuffix(sys.Name, "-REQUIRED"), "-ROLE"))
		} else {
			s.Kind = T(lang, "fields.type."+t)
		}
		out = append(out, s)
	}
	return out
}
