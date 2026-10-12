package main

import (
	"os"
	"strings"
	"testing"
)

// Integration test, sends real mails:
// MSERVER= MUSER= MPASS= MTO= go test -run TestSendMail
func TestSendMail(t *testing.T) {
	server, user, pass, to := os.Getenv("MSERVER"), os.Getenv("MUSER"), os.Getenv("MPASS"), os.Getenv("MTO")
	if server == "" {
		t.Skip("set MSERVER, MUSER, MPASS and MTO to send test mails")
	}
	conf := MailConfig{Server: server, User: user, Pass: pass, Port: 587}
	tls := conf
	tls.Port = 465

	plain := Mail{
		From:    user,
		To:      []string{to},
		Subject: "GoTests: Test mail subject",
		Text:    "Nice plain text body here.\n\nHave nice day!\nYours golang testing library",
	}
	html := plain
	html.Text = "<html><body><h1>Hi!</h1><p>This is HTML mail!<br></p><div>See You!<br>golang testing library</div></body></html>"
	other := plain
	other.From = "someone@difdom.com"
	att := html
	att.Files = []string{"mail_test.go"}
	emb := att
	emb.Text = `<html><body><img width=100 height=100 id="1" src="cid:logo.png"><b>Hi!</b><p>This is HTML mail with attachment and embed!<br></p></body></html>`
	emb.Embedded = []string{"img/logo.png"}

	tests := []struct {
		name    string
		conf    MailConfig
		mail    Mail
		wantErr bool
	}{
		{"timeouting send", MailConfig{Server: "server.com", User: "a", Pass: "b"}, Mail{From: "a@a.a", To: []string{"to@a.a"}}, true},
		{"not existing srv", MailConfig{Server: "rntuylmj320n290n03k093km43209d2.com", User: "a", Pass: "b"}, Mail{From: "a@a.a", To: []string{"to"}}, true},
		{"simple submission send", conf, plain, false},
		{"simple tls send", tls, plain, false},
		{"html mail", conf, html, false},
		{"send as someone else", conf, other, false},
		{"html mail with attachments", conf, att, false},
		{"html mail with attachment and embedded img", conf, emb, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.conf.Send(tt.mail); (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMailHTML(t *testing.T) {
	for _, c := range []struct {
		in      string
		html    bool
		wrapped bool
	}{
		{"<p>Dzień dobry</p><p><br></p>", true, true},
		{"Do zapłaty:<img width=\"150\" src=\"cid:x.jpeg\" />", true, true},
		{"<html><body><p>x</p></body></html>", true, false},
		{"Cena < 100 Kč, a > 50", false, false},
		{"zwykły tekst", false, false},
	} {
		out, ok := mailHTML(c.in)
		if ok != c.html {
			t.Errorf("mailHTML(%q) html=%v, want %v", c.in, ok, c.html)
		}
		if got := strings.HasPrefix(out, mailHTMLHead); got != c.wrapped {
			t.Errorf("mailHTML(%q) wrapped=%v, want %v", c.in, got, c.wrapped)
		}
	}
}
