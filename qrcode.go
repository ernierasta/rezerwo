package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// qrcode related functions

func QRCode(s, imgpath string) error {
	qrc, err := qrcode.New(s)
	if err != nil {
		return fmt.Errorf("could not generate QRCode: %v", err)

	}

	// katalog qrcode/ nowej organizacji jeszcze nie istnieje
	if err := os.MkdirAll(filepath.Dir(imgpath), 0o755); err != nil {
		return fmt.Errorf("could not create QR dir: %v", err)
	}

	w, err := standard.New(imgpath)
	if err != nil {
		return fmt.Errorf("standard.New failed: %v", err)

	}

	// save file
	if err = qrc.Save(w); err != nil {
		return fmt.Errorf("could not save image: %v", err)
	}
	return nil
}

type QRPayment struct {
	AccountNr     string
	Amount        float64
	Currency      string
	Message       string
	VarSymbol     int64
	RecipientName string
}

func NewQRPayment(accountnr, name string, amount float64, currency, msg string, varsymbol int64) *QRPayment {
	return &QRPayment{
		AccountNr:     accountnr,
		Amount:        amount,
		Currency:      currency,
		Message:       msg,
		VarSymbol:     varsymbol,
		RecipientName: name,
	}
}

func (q *QRPayment) GenCode(imgpath string) error {
	s, err := q.spayd()
	if err != nil {
		return err
	}
	return QRCode(s, imgpath)
}

// spayd - treść kodu w formacie SPAYD (Short Payment Descriptor), np.
// SPD*1.0*ACC:CZ2806000000000000000123*AM:450.00*CC:CZK*MSG:PLATBA*X-VS:1234567890*
// Bez znaków diakrytycznych, wiadomość i odbiorca wielkimi literami.
func (q *QRPayment) spayd() (string, error) {
	acc := RemoveDiacritics(q.AccountNr)
	amount := fmt.Sprintf("%.2f", q.Amount)
	currency := RemoveDiacritics(q.Currency)
	msg := RemoveDiacritics(strings.ToUpper(q.Message))
	vs := strconv.FormatInt(q.VarSymbol, 10)
	name := RemoveDiacritics(strings.ToUpper(q.RecipientName))

	switch {
	case len(acc) > 46 || len(acc) < 5:
		return "", fmt.Errorf("spayd: wrong account number %q", acc)
	case len(amount) > 10:
		return "", fmt.Errorf("spayd: amount too long %q", amount)
	case len(currency) != 3 && len(currency) != 0:
		return "", fmt.Errorf("spayd: wrong currency %q", currency)
	case len(msg) > 60:
		return "", fmt.Errorf("spayd: message too long %q", msg)
	case len(vs) > 10:
		return "", fmt.Errorf("spayd: variable symbol too long %q", vs)
	case len(name) > 35:
		return "", fmt.Errorf("spayd: recipient name too long %q", name)
	}

	s := "SPD*1.0*ACC:" + acc + "*AM:" + amount + "*"
	for _, part := range [][2]string{{"CC", currency}, {"MSG", msg}, {"RN", name}, {"X-VS", vs}} {
		if part[1] != "" {
			s += part[0] + ":" + part[1] + "*"
		}
	}
	return s, nil
}

func RemoveDiacritics(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return strings.NewReplacer("ł", "l", "Ł", "L").Replace(result)
}
