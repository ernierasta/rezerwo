package main

// pay.go: dane do przelewu w mailach i tekście "Po zamówieniu" imprezy, dla
// tych, którzy nie zapłacą kodem QR. Te same dane co w kodzie QR
// (GenerateQRimg): konto, kwota, symbol zmienny i wiadomość dla odbiorcy
// (jej początek ustawia się w imprezie, events.pay_message).
//
// W szablonie: {{.PayInfo "Konto"}} - cała tabelka, albo pojedynczo
// {{.PayAccount "Konto"}}, {{.PayIBAN "Konto"}}, {{.PayVS "Konto"}},
// {{.PayMessage "Konto"}}. Kwota to {{.TotalPrice}}.

import (
	"fmt"
	"html/template"
	"log"
	"strconv"
	"strings"
)

// payMessage - wiadomość dla odbiorcy dokładnie tak, jak trafia do kodu QR
// (GenerateQRimg): początek_NAZWISKO_IMIĘ wielkimi literami bez znaków
// diakrytycznych, najwyżej 60 znaków (limit formatu SPAYD - dłuższa
// wiadomość psuła kod QR), żeby wpłaty z QR i ręczne wyglądały na wyciągu
// tak samo.
func payMessage(prefix, nameSurname string) string {
	prefix = strings.TrimSpace(prefix)
	msg := nameSurname
	if prefix != "" {
		msg = prefix + "_" + nameSurname
	}
	msg = strings.ToUpper(RemoveDiacritics(msg))
	if r := []rune(msg); len(r) > 60 {
		msg = string(r[:60])
	}
	return msg
}

// splitIBAN rozdziela zapis konta z kodu QR "IBAN+BIC" i usuwa spacje.
func splitIBAN(s string) (iban, bic string) {
	s = strings.Join(strings.Fields(s), "")
	iban, bic, _ = strings.Cut(s, "+")
	return strings.ToUpper(iban), strings.ToUpper(bic)
}

// czechAccount zamienia czeski IBAN na krajowy numer konta
// (CZ65 0800 000019 2000145399 -> 19-2000145399/0800). Dla innych IBAN-ów "".
func czechAccount(iban string) string {
	if len(iban) != 24 || !strings.HasPrefix(iban, "CZ") {
		return ""
	}
	for _, r := range iban[2:] {
		if r < '0' || r > '9' {
			return ""
		}
	}
	bank, prefix, number := iban[4:8], strings.TrimLeft(iban[8:14], "0"), strings.TrimLeft(iban[14:], "0")
	if number == "" {
		return ""
	}
	if prefix != "" {
		return prefix + "-" + number + "/" + bank
	}
	return number + "/" + bank
}

// ibanValid sprawdza sumę kontrolną IBAN-u (mod 97), żeby wyłapać literówki.
func ibanValid(iban string) bool {
	if len(iban) < 15 || len(iban) > 34 {
		return false
	}
	r := 0
	for _, c := range iban[4:] + iban[:4] {
		switch {
		case c >= '0' && c <= '9':
			r = (r*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			r = (r*100 + int(c-'A'+10)) % 97
		default:
			return false
		}
	}
	return r == 1
}

// groupIBAN - IBAN w czwórkach, czytelniej do przepisania.
func groupIBAN(iban string) string {
	var b strings.Builder
	for i, r := range iban {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// payAccount - konto do przelewu. W podglądzie konto może być jeszcze
// niewybrane - wtedy ok=false i szablon pokazuje znacznik.
func (o *OrderFuncs) payAccount(accountName string) (BankAccount, bool) {
	ba, err := o.DB.BankAccountGetByName(accountName, o.User.ID)
	if err != nil {
		if !o.Preview {
			log.Printf("OrderFuncs.Pay: no account %q, userID: %d(%s), %v", accountName, o.User.ID, o.User.URL, err)
		}
		return ba, false
	}
	return ba, true
}

func (o *OrderFuncs) payMissing(label string) template.HTML {
	if !o.Preview {
		return ""
	}
	return template.HTML(`<span style="border:1px dashed #b9c3cf;color:#6e7885;padding:0 6px">` +
		template.HTMLEscapeString(T(o.Lang, label)) + `</span>`)
}

// PayAccount - numer konta: czeski krajowy, a gdy się nie da, IBAN.
func (o *OrderFuncs) PayAccount(accountName string) template.HTML {
	ba, ok := o.payAccount(accountName)
	if !ok {
		return o.payMissing("pay.account")
	}
	iban, _ := splitIBAN(ba.IBAN)
	if acc := czechAccount(iban); acc != "" {
		return template.HTML(template.HTMLEscapeString(acc))
	}
	return template.HTML(template.HTMLEscapeString(groupIBAN(iban)))
}

// PayIBAN - IBAN w czwórkach, z BIC/SWIFT w nawiasie, jeśli konto go ma.
func (o *OrderFuncs) PayIBAN(accountName string) template.HTML {
	ba, ok := o.payAccount(accountName)
	if !ok {
		return o.payMissing("pay.iban")
	}
	iban, bic := splitIBAN(ba.IBAN)
	s := groupIBAN(iban)
	if bic != "" {
		s += " (BIC/SWIFT " + bic + ")"
	}
	return template.HTML(template.HTMLEscapeString(s))
}

// PayVS - symbol zmienny, pusty, gdy konto go nie ma.
func (o *OrderFuncs) PayVS(accountName string) template.HTML {
	ba, ok := o.payAccount(accountName)
	if !ok {
		return o.payMissing("pay.vs")
	}
	if !ba.VarSymbol.Valid || ba.VarSymbol.Int64 == 0 {
		return ""
	}
	return template.HTML(strconv.FormatInt(ba.VarSymbol.Int64, 10))
}

// PayMessage - wiadomość dla odbiorcy tak, jak zapisuje ją kod QR.
func (o *OrderFuncs) PayMessage(accountName string) template.HTML {
	_, ok := o.payAccount(accountName)
	if !ok {
		return o.payMissing("pay.message")
	}
	return template.HTML(template.HTMLEscapeString(o.payMessage()))
}

func (o *OrderFuncs) payMessage() string {
	return payMessage(o.payPrefix, o.Surname+"_"+o.Name)
}

// payAmount - kwota z waluty konta ("1 200 CZK").
func (o *OrderFuncs) payAmount(ba BankAccount) string {
	am, _, _ := strings.Cut(strings.TrimSpace(o.TotalPrice), " ")
	n, err := strconv.ParseInt(am, 10, 64)
	if err != nil {
		return o.TotalPrice
	}
	return FormatMoney(n, ba.Currency)
}

// PayInfo - tabelka z kompletem danych do przelewu. Konta są czeskie, więc
// bez IBAN-u (jest w {{.PayIBAN}}) - IBAN pojawia się tylko zamiast numeru
// konta, gdy konto nie jest czeskie. Etykiety w języku imprezy; style
// w atrybutach, bo klienty pocztowe gubią <style>.
func (o *OrderFuncs) PayInfo(accountName string) template.HTML {
	ba, ok := o.payAccount(accountName)
	if !ok {
		return o.payMissing("pay.info")
	}
	iban, _ := splitIBAN(ba.IBAN)
	account := czechAccount(iban)
	if account == "" {
		account = groupIBAN(iban)
	}

	type row struct{ label, value string }
	rows := []row{}
	add := func(label, value string) {
		if value != "" {
			rows = append(rows, row{T(o.Lang, label), value})
		}
	}
	add("pay.recipient", ba.RecipientName.String)
	add("pay.account", account)
	add("pay.amount", o.payAmount(ba))
	if ba.VarSymbol.Valid && ba.VarSymbol.Int64 != 0 {
		add("pay.vs", strconv.FormatInt(ba.VarSymbol.Int64, 10))
	}
	add("pay.message", o.payMessage())

	var b strings.Builder
	b.WriteString(`<table style="border-collapse:collapse;margin:4px 0">`)
	for _, r := range rows {
		fmt.Fprintf(&b, `<tr><td style="padding:2px 14px 2px 0;color:#6e7885;vertical-align:top">%s</td><td style="padding:2px 0"><b>%s</b></td></tr>`,
			template.HTMLEscapeString(r.label), template.HTMLEscapeString(r.value))
	}
	b.WriteString(`</table>`)
	return template.HTML(b.String())
}
