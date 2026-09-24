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
	"log"
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
}

// PERSEATSUFFIX - końcówka nazwy pola dodatków powtarzanego wg liczby miejsc.
const PERSEATSUFFIX = "-perseat"

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

// AddonAnswer to jedna odpowiedź gościa na pole dodatków.
type AddonAnswer struct {
	Field  AddonField
	Value  string
	Amount int64 // dla pól z ceną: ile sztuk
	// Repeat - numer powtórzenia dla pól "tyle razy, ile miejsc" (1, 2, ...);
	// 0 dla pól wypełnianych raz na zamówienie
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
func CollectAddonAnswers(fields []AddonField, form map[string][]string, seats int64) []AddonAnswer {
	var out []AddonAnswer

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

// AddonsTotal to suma dopłat z odpowiedzi.
func AddonsTotal(answers []AddonAnswer) int64 {
	var total int64
	for _, a := range answers {
		total += a.Sum()
	}
	return total
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
			answerID, err := db.FormAnswerAdd(&FormAnswer{
				Value:       ToNS(a.Value),
				FormFieldID: fieldID,
				FormID:      formID,
			})
			if err != nil {
				log.Printf("SaveOrderWithAddons: answer %q: %v", a.Field.Name, err)
				continue
			}
			// numer powtórzenia ("Kolacja 2") - FormAnswerAdd go nie zna
			if a.Repeat > 0 {
				if err := db.FormAnswerSetRepeat(answerID, a.Repeat); err != nil {
					log.Printf("SaveOrderWithAddons: repeat index: %v", err)
				}
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
