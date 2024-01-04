package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"golang.org/x/text/encoding/charmap"
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
	// strip diacritics and convert to ISO8859-1
	accountnr, err := q.normalize(q.AccountNr)
	if err != nil {
		return fmt.Errorf("q.CreateString: error normalizing %s, %v", q.AccountNr, err)
	}
	amounts := fmt.Sprintf("%.2f", q.Amount)
	amount, err := q.normalize(amounts)
	if err != nil {
		return fmt.Errorf("q.CreateString: error normalizing %f, %v", q.Amount, err)
	}
	currency, err := q.normalize(q.Currency)
	if err != nil {
		return fmt.Errorf("q.CreateString: error normalizing %s, %v", q.Currency, err)
	}
	msg, err := q.normalize(strings.ToUpper(q.Message))
	if err != nil {
		return fmt.Errorf("q.CreateString: error normalizing %s, %v", q.Message, err)
	}
	varsymbols := strconv.FormatInt(q.VarSymbol, 10)
	varsymbol, err := q.normalize(varsymbols)
	if err != nil {
		return fmt.Errorf("q.CreateString: error normalizing %d, %v", q.VarSymbol, err)
	}
	name, err := q.normalize(strings.ToUpper(q.RecipientName))
	if err != nil {
		return fmt.Errorf("q.CreateString: error normalizing %s, %v", q.RecipientName, err)
	}

	// validate if according to specs
	if len(accountnr) > 46 || len(accountnr) < 5 {
		return fmt.Errorf("q.CreateString: wrong accountnr length %s (len %d), ", accountnr, len(accountnr))
	}
	if len(amount) > 10 {
		return fmt.Errorf("q.CreateString: wrong amounts length %s (len %d), ", amount, len(amount))
	}
	if len(currency) != 3 && len(currency) != 0 {
		return fmt.Errorf("q.CreateString: wrong currency, must be 3, length %s (len %d), ", currency, len(currency))
	}
	if len(msg) > 60 {
		return fmt.Errorf("q.CreateString: wrong msg length %s (len %d), ", msg, len(msg))
	}
	if len(varsymbol) > 10 {
		return fmt.Errorf("q.CreateString: wrong varsymbol length %s (len %d), ", varsymbol, len(varsymbol))
	}
	if len(name) > 35 {
		return fmt.Errorf("q.CreateString: wrong recipient name length %s (len %d), ", name, len(name))
	}

	//prepare segments
	var sname, samount, scurrency, smsg, svarsymbol string
	if name != "" {
		sname = fmt.Sprintf("RN:%s*", name)
	}
	if amount != "" {
		samount = fmt.Sprintf("AM:%s*", amount)
	}
	if currency != "" {
		scurrency = fmt.Sprintf("CC:%s*", currency)
	}
	if msg != "" {
		smsg = fmt.Sprintf("MSG:%s*", msg)
	}
	if varsymbol != "" {
		svarsymbol = fmt.Sprintf("X-VS:%s*", varsymbol)
	}

	// SPD*1.0*ACC:CZ2806000000000000000123*AM:450.00*CC:CZK*MSG:PLATBA ZA ZBOZI*X-VS:1234567890
	qrstring := fmt.Sprintf("SPD*1.0*ACC:%s*%s%s%s%s%s", accountnr, samount, scurrency, smsg, sname, svarsymbol)
	return QRCode(qrstring, imgpath)
}

func (q *QRPayment) normalize(s string) (string, error) {
	s = RemoveDiacritics(s)
	//return EncodeToISO8859_1(s)
	return s, nil
}

func RemoveDiacritics(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	result = strings.Replace(result, "ł", "l", -1)
	result = strings.Replace(result, "Ł", "L", -1)
	return result
}

func EncodeToISO8859_1(s string) ([]byte, error) {
	encoder := charmap.ISO8859_1.NewEncoder()
	out, err := encoder.Bytes([]byte(s))
	if err != nil {
		return out, err
	}
	return out, nil
}
