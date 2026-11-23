package main

// addons.go: dodatki do zamówienia - czytanie definicji pól i liczenie dopłat.
//
// Definicja dodatków to szablon formularza rodzaju "order_addons" przypięty do
// imprezy (events.addons_formtemplate_id). Pola zapisuje formBuilder w tym
// samym formacie JSON co zwykłe formularze, więc czytamy je tolerancyjnie:
// bierzemy to, co znamy, reszta nam nie przeszkadza.

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)

func nowUnix() int64 { return time.Now().Unix() }

// AddonOption to jedna pozycja listy wyboru.
type AddonOption struct {
	Label    string
	Value    string
	Selected bool
}

// AddonField to pole dodatków gotowe do wyświetlenia gościowi.
type AddonField struct {
	Type     string
	Name     string
	Label    string
	Required bool
	Options  []AddonOption

	// Pola typu "Wielokrotność" (multiplierField) działają jak dopłata:
	// gość podaje liczbę sztuk, cena jednej sztuki jest w definicji.
	Price  int64
	Before string
	After  string

	// PerSeat - pole powtarzane tyle razy, ile zarezerwowano miejsc
	// ("Kolacja 1", "Kolacja 2", ...). Włącza je końcówka nazwy pola
	// PERSEATSUFFIX, tak jak istniejąca konwencja "-REQUIRED" w formularzach.
	PerSeat bool

	// Multi - gość sam dokłada kolejne wpisy przyciskiem "+" (np. nazwiska
	// rodzin, dla których zamawia), najwyżej tyle, ile zarezerwował miejsc.
	// Włącza je końcówka nazwy MULTISUFFIX. Tylko pola bez ceny.
	Multi bool
	// MultiEmpty - na start bez pola, tylko przycisk (gość dodaje pierwszy
	// wpis sam); MultiButton - własny napis przycisku zamiast "Dodaj kolejną".
	// W JSON pola: multi_empty, multi_btn.
	MultiEmpty  bool
	MultiButton string
}

// AddonsPositions - gdzie w kroku zamówienia mogą stać dodatki: po którym
// polu danych osoby (events.addons_after, tmpl/n_order.html). Pusty = na
// końcu, po uwagach. Imię i nazwisko stoją w jednym wierszu, więc są jedną
// pozycją.
var AddonsPositions = []string{"start", "email", "name", "phone", ""}

// AddonsAfterNorm zwraca znaną pozycję dodatków (nieznana = na końcu).
func AddonsAfterNorm(s string) string {
	for _, p := range AddonsPositions {
		if s == p {
			return p
		}
	}
	return ""
}

// PERSEATSUFFIX - końcówka nazwy pola dodatków powtarzanego wg liczby miejsc.
const PERSEATSUFFIX = "-perseat"

// MULTISUFFIX - końcówka nazwy pola dodatków, które gość powtarza sam ("+").
const MULTISUFFIX = "-multi"

// IsPriced mówi, czy pole zwiększa kwotę zamówienia.
func (f AddonField) IsPriced() bool {
	return f.Type == "multiplierField" && f.Price > 0
}

// ParseAddonFields czyta definicję pól z formtemplates.content.
func ParseAddonFields(content string) []AddonField {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		log.Printf("ParseAddonFields: %v", err)
		return nil
	}

	var out []AddonField
	for _, m := range raw {
		f := AddonField{
			Type:     str(m["type"]),
			Name:     str(m["name"]),
			Label:    plainLabel(str(m["label"])),
			Required: boolOf(m["required"]),
		}
		if isTypeIgnored(f.Type) || f.Name == "" {
			continue // nagłówki, akapity, przyciski
		}
		f.PerSeat = strings.HasSuffix(strings.ToLower(f.Name), PERSEATSUFFIX)

		if vals, ok := m["values"].([]interface{}); ok {
			for _, v := range vals {
				vm, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				f.Options = append(f.Options, AddonOption{
					Label:    str(vm["label"]),
					Value:    str(vm["value"]),
					Selected: boolOf(vm["selected"]),
				})
			}
		}

		// multiplierField: liczba razy cena, z tekstem przed i po
		f.Price = intOf(m["multipl"])
		f.Multi = strings.HasSuffix(strings.ToLower(f.Name), MULTISUFFIX) && !f.IsPriced()
		if f.Multi {
			f.MultiEmpty = boolOf(m["multi_empty"])
			f.MultiButton = strings.TrimSpace(str(m["multi_btn"]))
		}
		// formBuilder i designer pól zapisują teksty jako "pre"/"post"
		// (nawiasy z domyślnego "(" ... " Kč)" w kroku zamówienia są zbędne)
		f.Before = strings.Trim(str(m["pre"]), "( ")
		f.After = strings.Trim(str(m["post"]), ") ")
		if f.Before == "" && f.After == "" {
			f.Before, f.After = str(m["before"]), str(m["after"])
		}

		out = append(out, f)
	}
	return out
}

// EventAddonFields zwraca pola dodatków imprezy (puste, gdy dodatków nie ma).
func EventAddonFields(db *DB, event Event) []AddonField {
	if !event.AddonsFormTemplateID.Valid || event.AddonsFormTemplateID.Int64 == 0 {
		return nil
	}
	templ, err := db.FormTemplateGetByID(event.AddonsFormTemplateID.Int64, event.UserID)
	if err != nil {
		log.Printf("EventAddonFields: template %d: %v", event.AddonsFormTemplateID.Int64, err)
		return nil
	}
	return ParseAddonFields(templ.Content.String)
}

// pricedAddonLabels zwraca etykiety pól dodatków, które mają cenę. Takie pola
// pokazujemy w podsumowaniu jako dopłaty (ile sztuk, za ile), a nie jako
// listę odpowiedzi - inaczej byłyby liczone dwa razy.
func pricedAddonLabels(db *DB, event Event) map[string]bool {
	out := map[string]bool{}
	for _, f := range EventAddonFields(db, event) {
		if f.IsPriced() {
			out[f.Label] = true
		}
	}
	return out
}

// OrderAddonLine to odpowiedzi jednego pola dodatków w jednym zamówieniu,
// np. "Rodziny: Kowalscy, Rusnokowie" albo "Kolacja: schabowy, ryba".
type OrderAddonLine struct {
	Field  AddonField
	Values []string
}

// addonAnswersByOrder zbiera odpowiedzi dodatków (bez pól z ceną - te są
// pozycjami zamówienia) po id zamówienia, pola w kolejności z definicji.
// Odpowiedzi na pola, których w definicji już nie ma, idą na koniec.
func addonAnswersByOrder(db *DB, event Event) map[int64][]OrderAddonLine {
	rows, err := db.AddonAnswersByEvent(event.ID)
	if err != nil {
		log.Printf("addonAnswersByOrder: %v", err)
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	return groupAddonAnswers(rows, EventAddonFields(db, event))
}

// groupAddonAnswers składa odpowiedzi w linie po zamówieniu i polu.
func groupAddonAnswers(rows []AddonAnswerRow, fields []AddonField) map[int64][]OrderAddonLine {
	pos := map[string]int{}
	for i, f := range fields {
		pos[f.Name] = i
	}

	out := map[int64][]OrderAddonLine{}
	for _, r := range rows {
		if strings.TrimSpace(r.Value) == "" {
			continue
		}
		var f AddonField
		if i, ok := pos[r.Name]; ok {
			f = fields[i]
		} else {
			f = AddonField{Name: r.Name, Label: plainLabel(r.Display)}
		}
		if f.IsPriced() {
			continue
		}
		lines := out[r.OrderID]
		found := false
		for i := range lines {
			if lines[i].Field.Name == f.Name {
				lines[i].Values = append(lines[i].Values, r.Value)
				found = true
				break
			}
		}
		if !found {
			lines = append(lines, OrderAddonLine{Field: f, Values: []string{r.Value}})
		}
		out[r.OrderID] = lines
	}

	// kolejność pól jak w definicji (nieznane na końcu)
	rank := func(name string) int {
		if i, ok := pos[name]; ok {
			return i
		}
		return len(fields)
	}
	for id, lines := range out {
		sort.SliceStable(lines, func(a, b int) bool {
			return rank(lines[a].Field.Name) < rank(lines[b].Field.Name)
		})
		out[id] = lines
	}
	return out
}

// orderAddonsHTML - dodatki zamówień do maila ({{.Addons}}): odpowiedzi
// ("Rodziny: Kowalscy, Rusnokowie") i pozycje z ceną ("Parking: 2 × 100").
// Bez dodatków pusto, więc linijka w mailu po prostu nic nie wstawia.
func orderAddonsHTML(db *DB, event Event, orderIDs []int64) template.HTML {
	esc := template.HTMLEscapeString
	fields := EventAddonFields(db, event)
	var b strings.Builder
	line := func(label, value string) {
		b.WriteString(esc(label) + ": <strong>" + esc(value) + "</strong><br>")
	}
	for _, id := range orderIDs {
		rows, err := db.AddonAnswersByOrder(id)
		if err != nil {
			log.Printf("orderAddonsHTML: order %d: %v", id, err)
		}
		for _, l := range groupAddonAnswers(rows, fields)[id] {
			line(l.Field.Label, strings.Join(l.Values, ", "))
		}
		items, err := db.OrderItemsGetByOrder(id)
		if err != nil {
			log.Printf("orderAddonsHTML: items %d: %v", id, err)
		}
		for _, it := range items {
			label, value, ok := strings.Cut(it.Description, ": ")
			if !ok {
				label, value = it.Description, fmt.Sprintf("%d × %d", it.Amount, it.Price)
			}
			line(label, value)
		}
	}
	return template.HTML(b.String())
}

// sampleAddonsHTML - przykład do podglądu maila: pola dodatków imprezy
// z wielokropkiem zamiast odpowiedzi.
func sampleAddonsHTML(db *DB, event Event, lang string) template.HTML {
	esc := template.HTMLEscapeString
	var b strings.Builder
	for _, f := range EventAddonFields(db, event) {
		v := "…"
		if f.IsPriced() {
			v = fmt.Sprintf("1 × %d", f.Price)
		}
		b.WriteString(esc(f.Label) + ": <strong>" + esc(v) + "</strong><br>")
	}
	if b.Len() == 0 {
		b.WriteString(esc(T(lang, "mail.sample.addons")))
	}
	return template.HTML(b.String())
}

// AddonAnswer to jedna odpowiedź gościa na pole dodatków.
type AddonAnswer struct {
	Field  AddonField
	Value  string
	Amount int64 // dla pól z ceną: ile sztuk
	// Repeat - numer powtórzenia dla pól "tyle razy, ile miejsc" i pól
	// dokładanych przez gościa (1, 2, ...); 0 dla pól wypełnianych raz
	Repeat int64
}

// Sum zwraca dopłatę z tej odpowiedzi.
func (a AddonAnswer) Sum() int64 {
	if !a.Field.IsPriced() {
		return 0
	}
	return a.Amount * a.Field.Price
}

// Description opisuje pozycję na zamówieniu, np. "Dodatkowa porcja: 2 × 85".
func (a AddonAnswer) Description() string {
	label := a.Field.Label
	if a.Repeat > 0 {
		label = fmt.Sprintf("%s %d", label, a.Repeat)
	}
	if a.Field.IsPriced() {
		return fmt.Sprintf("%s: %d × %d", label, a.Amount, a.Field.Price)
	}
	return fmt.Sprintf("%s: %s", label, a.Value)
}

// AddonInputName zwraca nazwę pola w formularzu zamówienia.
// Pola powtarzane mają numer powtórzenia w nazwie: addon-kolacja-perseat-2.
func AddonInputName(f AddonField, repeat int64) string {
	if repeat > 0 {
		return fmt.Sprintf("addon-%s-%d", f.Name, repeat)
	}
	return "addon-" + f.Name
}

// CollectAddonAnswers wyciąga odpowiedzi z formularza zamówienia.
// Pola mają w formularzu nazwy "addon-<nazwa pola>" (i "addon-<nazwa>-amount"
// dla liczby sztuk), żeby nie mieszać ich z danymi osoby. Pola powtarzane wg
// liczby miejsc mają numer powtórzenia w nazwie (addon-<nazwa>-1, -2, ...).
// Pola dokładane przez gościa ("+") też, ale czytamy najwyżej tyle wpisów, ile
// miejsc, puste pomijamy i numerujemy od nowa bez dziur.
func CollectAddonAnswers(fields []AddonField, form map[string][]string, seats int64) []AddonAnswer {
	var out []AddonAnswer

	collectMulti := func(f AddonField) {
		var n int64
		for i := int64(1); i <= seats; i++ {
			value := strings.TrimSpace(first(form[AddonInputName(f, i)]))
			if value == "" {
				continue
			}
			n++
			out = append(out, AddonAnswer{Field: f, Value: value, Repeat: n})
		}
	}

	collect := func(f AddonField, repeat int64) {
		name := AddonInputName(f, repeat)
		value := first(form[name])
		amount := intFromString(first(form[name+"-amount"]))

		if f.IsPriced() {
			if amount <= 0 {
				return // nie zamówiono sztuk
			}
			out = append(out, AddonAnswer{
				Field: f, Value: strconv.FormatInt(amount, 10), Amount: amount, Repeat: repeat,
			})
			return
		}
		if strings.TrimSpace(value) == "" {
			return
		}
		out = append(out, AddonAnswer{Field: f, Value: value, Repeat: repeat})
	}

	for _, f := range fields {
		if f.Multi {
			collectMulti(f)
			continue
		}
		if f.PerSeat && seats > 0 {
			for i := int64(1); i <= seats; i++ {
				collect(f, i)
			}
			continue
		}
		collect(f, 0)
	}
	return out
}

// SaveOrderWithAddons zakłada zamówienie dla złożonej rezerwacji: spina
// rezerwacje miejsc z klientem (tabela orders) i zapisuje dodatki - odpowiedzi
// jako zwykły formularz (forms + formanswers), a dopłaty jako pozycje
// zamówienia (order_items). Miejsca zostają w reservations.status.
//
// Zwraca id zamówienia i sumę dopłat. Błędy tylko loguje: zamówienie miejsc
// jest już zapisane i nie chcemy go przerywać z powodu dodatków.
func SaveOrderWithAddons(db *DB, event Event, user User, customerID int64, reservationIDs []int64, notes string, form map[string][]string) (int64, int64) {
	order := &CustomerOrder{
		CreatedDate: nowUnix(),
		Notes:       ToNS(notes),
		EventID:     event.ID,
		CustomerID:  customerID,
	}
	orderID, err := db.OrderAdd(order)
	if err != nil {
		log.Printf("SaveOrderWithAddons: order: %v", err)
		return 0, 0
	}

	for _, rid := range reservationIDs {
		if err := db.ReservationSetOrder(rid, orderID); err != nil {
			log.Printf("SaveOrderWithAddons: reservation %d: %v", rid, err)
		}
	}

	fields := EventAddonFields(db, event)
	if len(fields) == 0 {
		return orderID, 0
	}
	answers := CollectAddonAnswers(fields, form, int64(len(reservationIDs)))
	if len(answers) == 0 {
		return orderID, 0
	}

	// odpowiedzi trzymamy tak samo jak odpowiedzi formularzy, żeby raporty
	// i funkcje szablonów maili działały bez zmian
	templID := event.AddonsFormTemplateID.Int64
	formID, err := db.FormAdd(&Form{
		CreatedDate:    nowUnix(),
		UserID:         user.ID,
		FormTemplateID: templID,
		OrderID:        ToNI(orderID),
	})
	if err != nil {
		log.Printf("SaveOrderWithAddons: form: %v", err)
	} else {
		// FormAdd nie zna kolumny orders_id_fk (stare zapytanie), więc
		// powiązanie z zamówieniem dopisujemy osobno
		if err := db.FormSetOrder(formID, orderID); err != nil {
			log.Printf("SaveOrderWithAddons: form %d -> order %d: %v", formID, orderID, err)
		}
		for _, a := range answers {
			fieldID, err := db.FormFieldGetIDByName(a.Field.Name, templID)
			if err != nil {
				log.Printf("SaveOrderWithAddons: field %q: %v", a.Field.Name, err)
				continue
			}
			// numer powtórzenia ("Kolacja 2"), 0 dla zwykłych pól
			if _, err := db.FormAnswerAdd(&FormAnswer{
				Value:       ToNS(a.Value),
				FormFieldID: fieldID,
				FormID:      formID,
				RepeatIndex: ToNI(a.Repeat),
			}); err != nil {
				log.Printf("SaveOrderWithAddons: answer %q: %v", a.Field.Name, err)
			}
		}
	}

	var total int64
	for _, a := range answers {
		if !a.Field.IsPriced() {
			continue
		}
		total += a.Sum()
		if _, err := db.OrderItemAdd(&OrderItem{
			Kind:        "addon",
			Description: a.Description(),
			Amount:      a.Amount,
			Price:       a.Field.Price,
			Currency:    ToNS(event.DefaultCurrency),
			Status:      "ordered",
			CreatedDate: nowUnix(),
			OrderID:     orderID,
		}); err != nil {
			log.Printf("SaveOrderWithAddons: order item: %v", err)
		}
	}
	return orderID, total
}

// --- drobne pomoce ---------------------------------------------------------

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

func boolOf(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true" || b == "1"
	}
	return false
}

func intOf(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case string:
		return intFromString(n)
	}
	return 0
}

func intFromString(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func first(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}
