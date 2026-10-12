package main

import (
	"crypto/tls"
	"fmt"
	"log"
	gomail "net/mail"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/Necoro/html2text"
	"github.com/wneessen/go-mail"
)

// MailConfig - serwer SMTP z config.toml
type MailConfig struct {
	Server     string
	Port       int
	User, Pass string
	IgnoreCert bool
	From       string // nadawca maili systemowych (powiadomienia dla organizatorów)
}

// Mail - jedna wiadomość. Text z tagami HTML (z edytora maili) idzie jako HTML
// z automatyczną wersją tekstową, inaczej jako zwykły tekst.
type Mail struct {
	From     string
	To       []string
	Subject  string
	Text     string
	Files    []string // załączniki (ścieżki)
	Embedded []string // obrazki wstawiane jako cid:<nazwa pliku>
}

// Send wysyła mail przez SMTP (STARTTLS/TLS wymagane).
func (c MailConfig) Send(m Mail) error {
	if c.Server == "" || (c.User == "") != (c.Pass == "") {
		return fmt.Errorf("MailSend: incomplete SMTP config (server %q, user %q), sending aborted", c.Server, c.User)
	}
	msg, err := m.message()
	if err != nil {
		return err
	}
	if err := mailDeliver(c, msg); err != nil {
		return err
	}
	log.Printf("Mail successfully delivered.")
	return nil
}

// message buduje wiadomość MIME (bez wysyłania).
func (m Mail) message() (*mail.Msg, error) {
	msg := mail.NewMsg(mail.WithEncoding(mail.EncodingB64))
	if from, err := gomail.ParseAddress(m.From); err == nil {
		msg.FromMailAddress(from)
	} else if err := msg.From(m.From); err != nil {
		return nil, fmt.Errorf("MailSend: invalid FROM %q: %v", m.From, err)
	}
	if err := msg.To(strings.Join(m.To, ", ")); err != nil {
		return nil, fmt.Errorf("MailSend: invalid TO %v: %v", m.To, err)
	}
	msg.Subject(m.Subject)
	msg.SetMessageID()
	msg.SetDate()

	if text, ok := mailHTML(m.Text); ok {
		plain, err := html2text.FromString(text, html2text.Options{PrettyTables: true})
		if err != nil {
			log.Printf("MailSend: HTML to text: %v", err)
		}
		msg.SetBodyString(mail.TypeTextPlain, plain)
		msg.AddAlternativeString(mail.TypeTextHTML, text)
	} else {
		msg.SetBodyString(mail.TypeTextPlain, m.Text)
	}
	for _, f := range m.Files {
		msg.AttachFile(f)
	}
	for _, f := range m.Embedded {
		msg.EmbedFile(f)
	}
	return msg, nil
}

// mailDeliver wysyła gotową wiadomość; testy podmieniają ją na przechwytywanie
// (captureMails w mail_test.go).
var mailDeliver = func(c MailConfig, msg *mail.Msg) error {
	opts := []mail.Option{
		mail.WithTimeout(2 * time.Minute), // długo, bo bilety PDF w załącznikach
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover), mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithUsername(c.User), mail.WithPassword(c.Pass),
		// nasz serwer ma certyfikat na inną nazwę, stąd MailIgnoreCert = true
		mail.WithTLSConfig(&tls.Config{InsecureSkipVerify: c.IgnoreCert}),
	}
	if c.Port > 0 { // domyślnie go-mail używa 587
		opts = append(opts, mail.WithPort(c.Port))
	}
	client, err := mail.NewClient(c.Server, opts...)
	if err != nil {
		return fmt.Errorf("MailSend: new client: %v", err)
	}
	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("MailSend: deliver: %v", err)
	}
	return nil
}

// htmlTag - tag HTML, jakie wstawia edytor maili
var htmlTag = regexp.MustCompile(`(?i)<(html|p|br|div|span|table|strong|b|i|em|u|a|img|h[1-6]|ul|ol|li)[\s>/]`)

// mailHTML - treść maila jako dokument HTML; ok=false dla zwykłego tekstu.
// Fragment z edytora (bez <html>) dostaje nagłówek mailHTMLHead, bez niego
// programy pocztowe pokazują go dosłownie, z tagami.
func mailHTML(s string) (string, bool) {
	if strings.Contains(strings.ToLower(s), "<html") {
		return s, true
	}
	if !htmlTag.MatchString(s) {
		return s, false
	}
	return mailHTMLHead + s + "</html>", true
}

// SendAsOrg wysyła mail w imieniu organizacji (nadawca: alt_email, a gdy go
// nie ma - email organizacji) na jeden adres.
func (c MailConfig) SendAsOrg(user User, to, subject, text string, files, imgs []string) error {
	if to == "" {
		return fmt.Errorf("MailSend: no recipient (anonymous form?), mail not sent")
	}
	return c.Send(Mail{
		From:     chooseEmail(user.Email, user.AltEmail.String),
		To:       []string{to},
		Subject:  subject,
		Text:     text,
		Files:    files,
		Embedded: imgs,
	})
}

// mailMedia - ścieżki plików z listy "a.png;b.pdf" zapisanej przy mailu
// (katalog media/<organizacja>/email)
func mailMedia(list, userURL string) []string {
	if list == "" {
		return nil
	}
	var paths []string
	for _, f := range strings.Split(list, ";") {
		// tylko pliki z media/<org>/email/ (jak cleanFileList przy zapisie)
		if f = strings.TrimSpace(f); f == "" || strings.ContainsAny(f, "/\\") || strings.HasPrefix(f, ".") {
			continue
		}
		paths = append(paths, path.Join(MEDIAROOT, userURL, MEDIAEMAILSUBDIR, f))
	}
	return paths
}

// Files - załączniki maila
func (n Notification) Files(user User) []string {
	return mailMedia(n.AttachedFilesDelimited.String, user.URL)
}

// Images - obrazki wstawione w treść maila
func (n Notification) Images(user User) []string {
	return mailMedia(n.EmbeddedImgsDelimited.String, user.URL)
}

// chooseEmail - use main mail if alt_email is not defined
func chooseEmail(main, alt string) string {
	if alt != "" {
		return alt
	}
	return main
}
