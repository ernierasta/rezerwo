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
}

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
		f.Before = str(m["before"])
		f.After = str(m["after"])

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

// AddonAnswer to jedna odpowiedź gościa na pole dodatków.
type AddonAnswer struct {
	Field  AddonField
	Value  string
	Amount int64 // dla pól z ceną: ile sztuk
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
	if a.Field.IsPriced() {
		return fmt.Sprintf("%s: %d × %d", a.Field.Label, a.Amount, a.Field.Price)
	}
	return fmt.Sprintf("%s: %s", a.Field.Label, a.Value)
}

// CollectAddonAnswers wyciąga odpowiedzi z formularza zamówienia.
// Pola mają w formularzu nazwy "addon-<nazwa pola>" (i "addon-<nazwa>-amount"
// dla liczby sztuk), żeby nie mieszać ich z danymi osoby.
func CollectAddonAnswers(fields []AddonField, form map[string][]string) []AddonAnswer {
	var out []AddonAnswer
	for _, f := range fields {
		value := first(form["addon-"+f.Name])
		amount := intFromString(first(form["addon-"+f.Name+"-amount"]))

		if f.IsPriced() {
			if amount <= 0 {
				continue // nie zamówiono sztuk
			}
			out = append(out, AddonAnswer{Field: f, Value: strconv.FormatInt(amount, 10), Amount: amount})
			continue
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		out = append(out, AddonAnswer{Field: f, Value: value})
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
	answers := CollectAddonAnswers(fields, form)
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
			if _, err := db.FormAnswerAdd(&FormAnswer{
				Value:       ToNS(a.Value),
				FormFieldID: fieldID,
				FormID:      formID,
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
