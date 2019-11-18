package main

import (
	"encoding/base64"
	"log"

	"github.com/BurntSushi/toml"
)

type Settings struct {
	Location                string
	AuthenticationKeyBase64 string `toml:"AuthenticationKey"`
	AuthenticationKey       []byte
	EncryptionKeyBase64     string `toml:"EncryptionKey"`
	EncryptionKey           []byte
	DBFile                  string
	MailFrom                string
	MailUser                string
	MailPass                string
	MailServer              string
	MailPort                int64
	MailIgnoreCert          bool
}

func SettingsNew() *Settings {
	return &Settings{}
}

func (s *Settings) validate(f string) {
	if s.AuthenticationKeyBase64 == "" {
		log.Fatalf("error: in %q, AuthenticationKey is empty", f)
	}
	if s.EncryptionKeyBase64 == "" {
		log.Fatalf("error: in %q, EncryptionKey is empty", f)
	}

}

func (s *Settings) Read(f string) {
	var err error
	if _, err = toml.DecodeFile(f, &s); err != nil {
		log.Fatal(err)
	}
	s.validate(f)
	// Decode cookie session keys from base64
	if s.AuthenticationKey, err = base64.StdEncoding.DecodeString(s.AuthenticationKeyBase64); err != nil {
		log.Fatal(err)
	}
	if s.AuthenticationKey, err = base64.StdEncoding.DecodeString(s.AuthenticationKeyBase64); err != nil {
		log.Fatal(err)
	}
}
