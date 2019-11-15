package main

import (
	"log"

	"github.com/BurntSushi/toml"
)

type Settings struct {
	Location       string
	SessionKey     string
	DBFile         string
	MailFrom       string
	MailUser       string
	MailPass       string
	MailServer     string
	MailPort       int64
	MailIgnoreCert bool
}

func SettingsNew() *Settings {
	return &Settings{}
}

func (s *Settings) Validate() error {
	return nil
}

func (s *Settings) Read(f string) {
	if _, err := toml.DecodeFile(f, &s); err != nil {
		log.Fatal(err)
	}
}
