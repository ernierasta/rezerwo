package main

import (
	"os"
	"testing"
)

// not unit test, this is integration test
// to run it: invoke it like that:
// MSERVER= MUSER= MPASS= MTO= go test

var (
	server        = os.ExpandEnv("$MSERVER")
	user          = os.ExpandEnv("$MUSER")
	pass          = os.ExpandEnv("$MPASS")
	to            = []string{os.ExpandEnv("$MTO")}
	n, n2, n3, n4 MailConfig
)

func Init() {
	t := testing.T{}
	if server == "" {
		t.Fatalf("you have to set enviroment vars, invoke it like this:\nMSERVER= MUSER= MPASS= MTO= go test\n")
	}

	n = MailConfig{
		Server:  server,
		User:    user,
		Pass:    pass,
		Port:    587,
		To:      to,
		From:    user,
		Subject: "GoTests: Test mail subject",
		Text:    "Nice plain text body here.\n\nHave nice day!\nYours golang testing library",
	}
	n2 = n
	n2.Port = 465

	n3 = n
	n3.From = "someone@difdom.com"
	n3.Sender = user
	n3.ReplyTo = "someone@difdom.com"

	n4 = n
	n4.Text = "<html><body><h1>Hi!</h1><p>This is HTML mail!<br></p><div>See You!<br>golang testing library</div></body></html>"
}

func TestMail_Send(t *testing.T) {
	Init()
	type args struct {
		n MailConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"timeouting send", args{MailConfig{Server: "server.com", User: "a", Pass: "b", To: []string{"to"}}}, true},
		{"not existing srv", args{MailConfig{Server: "rntuylmj320n290n03k093km43209d2.com", User: "a", Pass: "b", To: []string{"to"}}}, true},
		{"simple submission send", args{n}, false},
		{"simple tls send", args{n2}, false},
		{"html mail", args{n4}, false},
		{"send as someone else", args{n3}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.args.n)
			if err := MailSend(tt.args.n); (err != nil) != tt.wantErr {
				t.Errorf("MailSend() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				t.Log(err)
			}
		})
	}
}
