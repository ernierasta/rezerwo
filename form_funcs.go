package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"path"
	"sort"
	"strconv"
	"strings"
)

// parseFormMailTemplate wypełnia treść maila danymi odpowiedzi; zwraca tekst
// i obrazki QR do wstawienia w mail
func parseFormMailTemplate(name, surname, mtext string, formID int64, user User, formTempl FormTemplate, db *DB) (string, []string) {
	ff := &FormFuncs{DB: db, User: user, Template: formTempl, FormID: formID, NameSurname: surname + "_" + name}
	tmpl, err := template.New("thankyou").Parse(mtext)
	if err != nil {
		log.Printf("parseFormMailTemplate: parse: %v", err)
		return "", nil
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ff); err != nil {
		log.Printf("parseFormMailTemplate: exec: %v", err)
	}
	return buf.String(), ff.EmbeddedImgs
}

type FormFuncs struct {
	DB          *DB
	User        User
	FormID      int64
	Template    FormTemplate
	NameSurname string
	// EmbeddedImgs - obrazki (QR) użyte w mailu jako cid:, wysyłający
	// musi je dołączyć
	EmbeddedImgs []string

	// answers - odpowiedzi wczytane raz dla .Summary i .OptionCount (form_stats.go)
	answers formAnswers
}

// Sum returns sum of given form field.
// form field can be specified by name or display value,
// where name is always checked first.
// This function is forms specific!
func (i *FormFuncs) Sum(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, false, false, "")
}

// SumAlfa sorts results by name (not by ammount) in case of string values
func (i *FormFuncs) SumAlfa(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, true, false, "")
}

// SumStrings forces string values (even when they are numbers) and sorts by string name
func (i *FormFuncs) SumStrings(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, true, true, "")
}

// SumOnly returns numerical output, it counts only given string as +1.
// Usefull for checkbox type with only one answer ("yes" for example)
// or for radio grups with "yes" and "no" options ("no" is ignored).
func (i *FormFuncs) SumOnly(FormFieldName, countOnlyThis string) template.HTML {
	return i.sum(FormFieldName, false, false, countOnlyThis)
}

func (i *FormFuncs) sum(FormFieldName string, sortByName, forceStringVals bool, countOnlyThis string) template.HTML {
	var sum int64

	// special value "forms" - counts how many forms are filled
	if strings.ToLower(FormFieldName) == "forms" {
		amm, err := i.DB.FormGetAmmount(i.User.ID, i.Template.ID)
		if err != nil {
			log.Printf("Sum: error getting ammount of forms for user %q, formTemplID %d, %v", i.User.URL, i.Template.ID, err)
		}
		return template.HTML(strconv.FormatInt(amm, 10))
	}

	id := i.fieldID(FormFieldName)

	if !forceStringVals { // if strings are forced, skip int retrieval
		// try to get ints from db, if can not do that assume strings
		if nrs, err := i.DB.FormAnswerGetAllAnswersForFieldInts(id); err == nil { // there are numbers, so return ordinary sum
			for i := range nrs {
				sum += nrs[i]
			}
			return template.HTML(strconv.FormatInt(sum, 10))
		}
	}

	// Let's sum string answers, we are counting the same string separated by ', ' across
	// all answers in given field
	strs, err := i.DB.FormAnswerGetAllAnswersForFieldStrings(id)
	if err != nil {
		log.Printf("Sum: answers of field %s(id:%d): %v", FormFieldName, id, err)
	}

	// there is special kind of string - miltiplication field, check for it
	// then it will be: 3 * 600 = 1200 and we are interested only in "3"
	if len(strs) > 0 {
		if strings.Contains(strs[0], " = ") && strings.Contains(strs[0], " * ") { // we are sure it is multiplictation
			sum := 0
			for i := range strs {
				am, _, _ := ConvertMultiplicationToAmmountMultiplTotal(strs[i])
				ami, err := strconv.Atoi(am)
				if err != nil {
					log.Printf("Sum: error converting multiplication field ammount %s into integer, %v", am, err)
				}
				sum += ami
			}
			return template.HTML(strconv.Itoa(sum))

		}
	}

	m := map[string]int{}
	for i := range strs {
		for _, v := range strings.Split(strs[i], ", ") {
			m[v]++
		}
	}

	// if countOnlyThis is defined, then return number for answer named as given
	// f.e.: we are interested in only "yes" values
	if len(countOnlyThis) != 0 {
		return template.HTML(strconv.Itoa(m[countOnlyThis]))
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// po nazwie albo od najczęstszej odpowiedzi (przy remisie po nazwie)
	sort.Slice(keys, func(a, b int) bool {
		if !sortByName && m[keys[a]] != m[keys[b]] {
			return m[keys[a]] > m[keys[b]]
		}
		return keys[a] < keys[b]
	})

	// odpowiedzi wpisują goście, a lista bywa na publicznej stronie formularza
	var out strings.Builder
	out.WriteString(`<ul class="sum-list">`)
	for _, k := range keys {
		fmt.Fprintf(&out, `<li class="sum-list-el">%s: <b>%d</b></li>`, template.HTMLEscapeString(k), m[k])
	}
	out.WriteString("</ul>")
	return template.HTML(out.String())
}

// fieldID - ID pola formularza po nazwie, a gdy jej nie ma - po etykiecie
func (i *FormFuncs) fieldID(name string) int64 {
	id, err := i.DB.FormFieldGetIDByName(name, i.Template.ID)
	if err != nil {
		id, _ = i.DB.FormFieldGetIDByDisplay(name, i.Template.ID)
	}
	return id
}

// Field shows form field value as is in db.
// This function is forms specific!
func (i *FormFuncs) Field(FormFieldName string) string {
	FieldID := i.fieldID(FormFieldName)
	s, err := i.DB.FormAnswerGetByField(i.FormID, FieldID)
	if err != nil {
		log.Printf("Field: can not get data for field %s(id:%d), formID: %d, %v", FormFieldName, FieldID, i.FormID, err)
	}

	return s
}

// This shows data from db or "alternative" string. Used for checkboxes with only one answer "yes", alternative will be "no".
// Usefull for mail notifications.
func (i *FormFuncs) FieldOr(FormFieldName, Alternative string) string {
	res := i.Field(FormFieldName)
	if res == "" {
		return Alternative
	}
	return res
}

// this functions allows to get separated data from multiplicateField (3 * 500 = 1500)
// MAmmount return ammount (of tickets usually)
func (i *FormFuncs) MAmmount(FormFieldName string) string {
	s := i.Field(FormFieldName)
	amm, _, _ := ConvertMultiplicationToAmmountMultiplTotal(s)
	return amm
}

// MMultipl returns money it is multiplied by
func (i *FormFuncs) MMultipl(FormFieldName string) string {
	s := i.Field(FormFieldName)
	_, mult, _ := ConvertMultiplicationToAmmountMultiplTotal(s)
	return mult
}

// MTotal return total price to be payed
func (i *FormFuncs) MTotal(FormFieldName string) string {
	s := i.Field(FormFieldName)
	_, _, total := ConvertMultiplicationToAmmountMultiplTotal(s)
	return total
}

// qr generuje kod QR płatności za odpowiedź; kwota z pola field (domyślnie
// z pola kwoty formularza), prefix rozróżnia kilka kodów w jednym mailu
func (i *FormFuncs) qr(accountName, field, prefix string) (string, string) {
	if field == "" {
		field = i.Template.MoneyAmountFieldName.String
	}
	am, err := i.DB.FormAnswerGetByFieldName(i.FormID, i.Template.ID, field)
	if err != nil {
		if am, err = i.DB.FormAnswerGetByFieldDisplay(i.FormID, i.Template.ID, field); err != nil {
			log.Printf("FormFuncs.qr: money field %q, form %d: %v", field, i.FormID, err)
		}
	}
	// pole "Wielokrotność" ma postać "3 * 500 = 1500"
	_, _, am = ConvertMultiplicationToAmmountMultiplTotal(am)

	name := strings.Join(strings.Fields(i.Template.URL), "") + "_"
	if prefix != "" {
		name += prefix + "_"
	}
	return qrPayImage(i.DB, i.User, accountName, name+i.NameSurname, am, payMessage(i.Template.PayMessage, i.NameSurname))
}

func (i *FormFuncs) QRPay(accountName string) template.HTML {
	_, p := i.qr(accountName, "", "")
	return qrImgHTML(p)
}

// QRPayFieldPrefix like QRPay, but allows to:
// customize field name (when multiple QRcodes should be generated),
// customize imgPrefix (avoid using the same qrcode file if multiple QRImages for one form).
func (i *FormFuncs) QRPayFieldPrefix(accountName, field, imgPrefix string) template.HTML {
	_, p := i.qr(accountName, field, imgPrefix)
	return qrImgHTML(p)
}

func (i *FormFuncs) QRPayMail(accountName string) template.HTML {
	name, p := i.qr(accountName, "", "")
	return embedImg(&i.EmbeddedImgs, name, p)
}

// QRPayMailFieldPrefix the same as QRPayFieldPrefix but for mail notifications
func (i *FormFuncs) QRPayMailFieldPrefix(accountName, field, imgPrefix string) template.HTML {
	name, p := i.qr(accountName, field, imgPrefix)
	return embedImg(&i.EmbeddedImgs, name, p)
}

// qrPayImage zapisuje kod QR płatności na konto accountName organizacji
// w media/<organizacja>/qrcode/<base>.jpeg; zwraca nazwę i ścieżkę pliku
func qrPayImage(db *DB, u User, accountName, base, amount, msg string) (string, string) {
	ba, err := db.BankAccountGetByName(accountName, u.ID)
	if err != nil {
		log.Printf("qrPayImage: no account %q for user %d(%s): %v", accountName, u.ID, u.URL, err)
	}
	// base zawiera imię i nazwisko gościa - bez separatorów ścieżki
	name := sanitize(base) + ".jpeg"
	p := path.Join(MEDIAROOT, u.URL, MEDIAQRCODESUBDIR, name)
	ami, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		log.Printf("qrPayImage: amount %q: %v", amount, err)
	}
	qr := NewQRPayment(ba.IBAN, ba.RecipientName.String, float64(ami), ba.Currency, msg, ba.VarSymbol.Int64)
	if err := qr.GenCode(p); err != nil {
		log.Printf("qrPayImage: %v", err)
	}
	return name, p
}

// qrImgHTML - kod QR na stronie
func qrImgHTML(p string) template.HTML {
	return template.HTML(fmt.Sprintf(`<img width="200" src="/%s" alt="QRError">`, template.HTMLEscapeString(p)))
}

// embedImg - obrazek w mailu (cid:), plik trafia do listy dołączanych
func embedImg(imgs *[]string, name, p string) template.HTML {
	*imgs = append(*imgs, p)
	return template.HTML(fmt.Sprintf(`<img width="150" src="cid:%s" alt="QR Kod" />`, template.HTMLEscapeString(name)))
}

// ConvertMultiplicationToAmmountMultiplTotal returns:
// ammount, multiplier, sum
func ConvertMultiplicationToAmmountMultiplTotal(s string) (string, string, string) {
	sum := ""
	ammount := ""
	multiplier := ""

	// if string doesn't contains "=" then it is ordinary ammount
	if strings.Contains(s, "=") {
		ss := strings.Split(s, "=")
		sum = strings.TrimSpace(ss[len(ss)-1])
		if strings.Contains(ss[0], "*") {
			sm := strings.Split(ss[0], "*")
			ammount = strings.TrimSpace(sm[0])
			multiplier = strings.TrimSpace(sm[1])
		} else {
			log.Printf("ConvertMultiplicationToAmmountMultiplSum: no '*' symbol in %q", s)
		}
	} else {
		return "", "", s // returning as is
	}
	return ammount, multiplier, sum
}
