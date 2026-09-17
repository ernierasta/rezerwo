package main

// Test zapisu zamówienia z dodatkami. Działa na KOPII bazy w tmp/, więc nie
// rusza db.sql. Uruchomienie: go test -run TestSaveOrderWithAddons
//
// Pełnej ścieżki /order/status nie da się przetestować automatycznie (wysyła
// maile), więc sprawdzamy samą funkcję zapisu.

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func copyDBForTest(t *testing.T) string {
	t.Helper()

	src, err := os.Open("db.sql")
	if err != nil {
		t.Skipf("brak db.sql, pomijam test: %v", err)
	}
	defer src.Close()

	if err := os.MkdirAll("tmp", 0o755); err != nil {
		t.Fatalf("tmp: %v", err)
	}
	path := filepath.Join("tmp", "test-addons.sql")
	dst, err := os.Create(path)
	if err != nil {
		t.Fatalf("kopia bazy: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		t.Fatalf("kopiowanie bazy: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(path)
		os.Remove(path + "-shm")
		os.Remove(path + "-wal")
	})
	return path
}

func TestSaveOrderWithAddons(t *testing.T) {
	path := copyDBForTest(t)

	db := DBInit(path)
	db.MustConnect()
	defer db.Close()

	// czy baza jest już po migracji dodatków
	var cnt int
	if err := db.DB.Get(&cnt, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='order_items'`); err != nil || cnt == 0 {
		t.Skip("kopia bazy bez migracji dodatków, pomijam test")
	}

	var user User
	if err := db.DB.Get(&user, `SELECT * FROM users LIMIT 1`); err != nil {
		t.Skipf("brak organizacji w bazie: %v", err)
	}

	// impreza, definicja dodatków i pola tylko na potrzeby testu
	eventID, err := db.EventAdd(&Event{Name: "__test addons__", UserID: user.ID, DefaultCurrency: "Kč"})
	if err != nil {
		t.Fatalf("impreza: %v", err)
	}
	templID, err := db.FormTemplateAdd(&FormTemplate{
		Name:        "__test addons tmpl__",
		URL:         "test-addons-tmpl",
		Content:     ToNS(addonsContent),
		CreatedDate: nowUnix(),
		UserID:      user.ID,
		EventID:     ToNI(eventID),
	})
	if err != nil {
		t.Fatalf("szablon dodatków: %v", err)
	}
	if err := db.FormTemplateSetKind(templID, user.ID, ADDONSKIND); err != nil {
		t.Fatalf("rodzaj szablonu: %v", err)
	}
	if err := db.EventSetAddonsTemplate(eventID, user.ID, ToNI(templID)); err != nil {
		t.Fatalf("przypięcie dodatków: %v", err)
	}
	for _, f := range []FormField{
		{Name: "obiad", Display: "Obiad", Type: "select", FormTemplateID: templID},
		{Name: "porcja", Display: "Dodatkowa porcja", Type: "multiplierField", FormTemplateID: templID},
	} {
		if _, err := db.FormFieldAdd(&f); err != nil {
			t.Fatalf("pole %q: %v", f.Name, err)
		}
	}

	customerID, err := db.CustomerAdd(&Customer{Email: "test-addons@example.invalid", Notes: ""})
	if err != nil {
		t.Fatalf("klient: %v", err)
	}

	// dwa miejsca w zamówieniu
	var resIDs []int64
	for i := 0; i < 2; i++ {
		id, err := db.ReservationAdd(&Reservation{
			OrderedDate: ToNI(nowUnix()),
			Price:       ToNI(750),
			Currency:    ToNS("Kč"),
			Status:      "ordered",
			FurnitureID: int64(900000 + i), // mebel nieistotny dla tego testu
			EventID:     eventID,
			CustomerID:  customerID,
		})
		if err != nil {
			t.Fatalf("rezerwacja: %v", err)
		}
		resIDs = append(resIDs, id)
	}

	event, err := db.EventGetByID(eventID)
	if err != nil {
		t.Fatalf("odczyt imprezy: %v", err)
	}

	form := map[string][]string{
		"addon-obiad":         {"wege"},
		"addon-porcja-amount": {"3"},
	}
	orderID, total := SaveOrderWithAddons(db, event, user, customerID, resIDs, "bez orzechów", form)

	if orderID == 0 {
		t.Fatal("zamówienie nie powstało")
	}
	if total != 255 { // 3 × 85
		t.Errorf("suma dopłat: %d, oczekiwano 255", total)
	}

	order, err := db.OrderGetByID(orderID, eventID)
	if err != nil {
		t.Fatalf("odczyt zamówienia: %v", err)
	}
	if order.CustomerID != customerID || order.Notes.String != "bez orzechów" {
		t.Errorf("zamówienie zapisane źle: %+v", order)
	}

	for _, id := range resIDs {
		var got int64
		if err := db.DB.Get(&got, `SELECT COALESCE(orders_id_fk, 0) FROM reservations WHERE id=$1`, id); err != nil {
			t.Fatalf("odczyt rezerwacji: %v", err)
		}
		if got != orderID {
			t.Errorf("rezerwacja %d nie jest spięta z zamówieniem (%d)", id, got)
		}
	}

	items, err := db.OrderItemsGetByOrder(orderID)
	if err != nil {
		t.Fatalf("pozycje zamówienia: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("oczekiwano 1 pozycji (tylko płatny dodatek), jest %d: %+v", len(items), items)
	}
	if items[0].Amount != 3 || items[0].Price != 85 || items[0].Status != "ordered" {
		t.Errorf("pozycja zapisana źle: %+v", items[0])
	}

	// odpowiedzi (także te bez ceny) zapisujemy jak zwykły formularz
	var answers int
	if err := db.DB.Get(&answers, `SELECT count(*) FROM formanswers fa
		JOIN forms f ON fa.forms_id_fk = f.id WHERE f.orders_id_fk = $1`, orderID); err != nil {
		t.Fatalf("odczyt odpowiedzi: %v", err)
	}
	if answers != 2 {
		t.Errorf("oczekiwano 2 odpowiedzi (obiad + porcja), jest %d", answers)
	}
}
