package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
)

// NotifConfig type represent all notification attributes
type MailConfig struct {
	Server          string
	Port            int
	User, Pass      string
	IgnoreCert      bool
	From            string
	Sender          string
	ReplyTo         string
	To              []string
	Subject         string
	Text            string
	Files           []string
	EmbededHTMLImgs []EmbImg
	Hostname        string
}

type EmbImg struct {
	NamePath string
	// CID must be in form myimage@domain.com
	CID string
}

/*
Correct mail stuct html with attachments:

From: x
To: y
...
Content-Type: multipart/mixed; boundary="boundary1"

--boundary1
Content-Type: multipart/alternative; boundary="boundary2"

--boundary2
Content-Type: text/plain; charset="UTF-8"

hi!

--boundary2
Content-Type: text/html; charset="UTF-8"

<html><body><p>hi</p></body></html>

--boundary2--
--boundary1
Content-Type: application/octet-stream; name="PLAN - I semestr(2).docx"
Content-Disposition: attachment; filename="PLAN - I semestr(2).docx"
Content-Transfer-Encoding: base64

aU...

--boundary1
Content-Type: application/vnd.oasis.opendocument.text; name="odziez.2.odt"
Content-Disposition: attachment; filename="odziez.2.odt"
Content-Transfer-Encoding: base64

aaUU ...
--boundary1--

*/

// MailSend sends mail via smtp.
// Supports multiple recepients, TLS (port 465)/StartTLS(ports 25,587, any other).
// Mail should always valid (correctly encoded subject and body).
// Now there is HTML (with automatic text version generating) support.
// We can also send attachments.
func MailSend(n MailConfig) error {

	var b0 = "00000000000035014305975deMmm"
	var b1 = "00000000000035014305975de62a"
	var b2 = "00000000000035014305975de6Xx"

	if (n.User != "" && n.Pass == "") ||
		(n.Pass != "" && n.User == "") ||
		n.Server == "" {
		pass := ""
		if len(n.Pass) > 3 {
			pass = n.Pass[0:3] + "..."
		} else {
			pass = n.Pass // if someone has 4 leter pass, it deserves to be logged ;-)
		}
		return fmt.Errorf("SendMail: one of auth params is empty(SENDING ABORTED), u: %q p:%q s: %q", n.User, pass, n.Server)
	}

	if n.Hostname == "" {
		return fmt.Errorf("SendMail: hostname is not defined, it is needed to generate unique Message-Id (SENDING ABORTED)")
	}

	auth := smtp.PlainAuth("", n.User, n.Pass, n.Server)

	recipients := strings.Join(n.To, ", ")

	subjectb64 := base64.StdEncoding.EncodeToString([]byte(n.Subject))

	// if from contains not only email, but something like "Name Surname <mail@something.com>"
	// we need to be sure international chars are properly encoded
	if strings.Contains(n.From, "<") {
		ss := strings.Split(n.From, "<")
		if len(ss) == 2 {
			m := mail.Address{Name: ss[0], Address: strings.TrimSuffix(ss[1], ">")}
			n.From = m.String()
		}
	}

	header := make(map[string]string)
	header["From"] = n.From
	header["To"] = recipients
	header["Date"] = time.Now().Format(time.RFC1123Z)
	header["Subject"] = "=?UTF-8?B?" + subjectb64 + "?="
	header["MIME-Version"] = "1.0"
	header["Message-Id"] = fmt.Sprintf("<%s>", generateMessageIDWithHostname(n.Hostname))

	if n.Sender != "" {
		header["Sender"] = n.Sender
	}
	// remove reply to - it can be problem for antispam
	//if n.ReplyTo != "" {
	//	header["Reply-To"] = n.ReplyTo
	//}
	isHTML := strings.Contains(n.Text, "<html>")
	hasAttachments := len(n.Files) > 0
	hasEmbeddedImgs := len(n.EmbededHTMLImgs) > 0

	msg := ""

	if isHTML {
		if hasAttachments {
			header["Content-Type"] = fmt.Sprintf("multipart/mixed;boundary=\"%s\"", b0)
			msg += "\r\n" + "--" + b0 + "\r\n"
		}
		if hasEmbeddedImgs {
			header["Content-Type"] = fmt.Sprintf("multipart/related;boundary=\"%s\"", b1)
			msg += "\r\n" + "--" + b1 + "\r\n"
		}

		// prepate "alternative" section plain/html
		altCont, err := alternativeContent("", n.Text, b2)
		if err != nil {
			return err
		}
		msg += altCont

		// attachments
		attachCont := ""
		for i := range n.Files {
			ct, err := validateFile(n.Files[i])
			if err != nil {
				return err
			}
			f, err := os.ReadFile(n.Files[i])
			if err != nil {
				return err
			}
			attachCont += "--" + b1 + "\r\n"
			attachCont += fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", ct, n.Files[i])
			attachCont += fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", n.Files[i])
			attachCont += "Content-Transfer-Encoding: base64\r\n"
			attachCont += "\r\n" + base64.StdEncoding.EncodeToString(f) + "\r\n"
		}
		if attachCont != "" {
			msg += attachCont
			msg += "--" + b0 + "--\r\n"
		}

		embededImgs := ""
		// add embeded images with CID
		for i := range n.EmbededHTMLImgs {
			ct, err := validateFile(n.EmbededHTMLImgs[i].NamePath)
			if err != nil {
				return err
			}
			f, err := os.ReadFile(n.EmbededHTMLImgs[i].NamePath)
			if err != nil {
				return err
			}
			embededImgs += "--" + b1 + "\r\n"
			embededImgs += fmt.Sprintf("Content-Type: %s\r\n", ct)
			embededImgs += fmt.Sprintf("Content-ID: <%s>\r\n", n.EmbededHTMLImgs[i].CID)
			embededImgs += fmt.Sprintf("Content-Disposition: inline; filename=\"%s\"\r\n", n.EmbededHTMLImgs[i].NamePath)
			embededImgs += "Content-Transfer-Encoding: base64\r\n"
			embededImgs += "\r\n" + base64.StdEncoding.EncodeToString(f) + "\r\n"
		}
		if embededImgs != "" {
			msg += embededImgs
			msg += "--" + b1 + "--\r\n"
		}
		// this no necessary imho
		//} else if isHTML && !hasAttachments {
		//		var err error
		//		msg, err = alternativeContent("", n.Text, b1)
		//		if err != nil {
		//			return err
		//		}
	} else {
		header["Content-Transfer-Encoding"] = "base64"
		header["Content-Type"] = "text/plain; charset=\"utf-8\""
		msg = base64.StdEncoding.EncodeToString([]byte(n.Text))
	}

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}

	message += msg
	//_ = auth
	err := sendMail(n.Server, n.Port, auth, n.IgnoreCert, n.From, n.To, []byte(message))

	if err != nil {
		return fmt.Errorf("SendMail: error sending mail, err: %v", err)
	}

	return nil
}

func encodeRFC2047(s string) string {
	// use mail's rfc2047 to encode any string
	addr := mail.Address{Name: s, Address: ""}
	return strings.Trim(addr.String(), " <@>")
}

// sendMail connects to the server at addr, switches to TLS if
// possible, authenticates with the optional mechanism a if possible,
// and then sends an email from address from, to addresses to, with
// message msg.
// The addr must include a port, as in "mail.example.com:smtp".
//
// The addresses in the to parameter are the SMTP RCPT addresses.
//
// The msg parameter should be an RFC 822-style email with headers
// first, a blank line, and then the message body. The lines of msg
// should be CRLF terminated. The msg headers should usually include
// fields such as "From", "To", "Subject", and "Cc".  Sending "Bcc"
// messages is accomplished by including an email address in the to
// parameter but not including it in the msg headers.
//
// The SendMail function and the net/smtp package are low-level
// mechanisms and provide no support for DKIM signing, MIME
// attachments (see the mime/multipart package), or other mail
// functionality. Higher-level packages exist outside of the standard
// library.
//
// sendMail ripped from net/smtp package, added ability to send mails
// via TLS (port: 465).
//
// Changes for zoriX:
//   - fixed potential MITM atack
//   - added option to ignore certificate
//   - fixed stuck connection if server has port closed (added timeout)
func sendMail(host string, port int, a smtp.Auth, ignoreCert bool, from string, to []string, msg []byte) error {
	if err := validateLine(from); err != nil {
		return err
	}

	if len(to) == 0 {
		return fmt.Errorf("smtp: no recepients given")
	}
	for _, recp := range to {
		if err := validateLine(recp); err != nil {
			return err
		}
	}

	hostPort := fmt.Sprintf("%s:%d", host, port)

	c := &smtp.Client{}

	// raw network connection with timeout
	netConn, err := net.DialTimeout("tcp", hostPort, 10*time.Second)
	if err != nil {
		return err
	}

	if port == 465 {
		tlsconfig := &tls.Config{
			InsecureSkipVerify: ignoreCert,
			ServerName:         host,
		}
		conn := tls.Client(netConn, tlsconfig)
		if err != nil {
			return err
		}
		c, err = smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
	} else { // using submission
		var err error
		c, err = smtp.NewClient(netConn, host)
		if err != nil {
			return err
		}
	}
	defer c.Close()
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	if err = c.Hello(hostname); err != nil {
		return err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{
			InsecureSkipVerify: ignoreCert,
			ServerName:         host,
		}
		if err = c.StartTLS(config); err != nil {
			return err
		}
	}
	if a != nil {
		if ok, _ := c.Extension("AUTH"); !ok {
			return fmt.Errorf("smtp: server doesn't support AUTH")
		}
		if err = c.Auth(a); err != nil {
			return err
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

func check(err error) {
	if err != nil {
		log.Println("error sending mail, err: ", err)
	}
}

// validateLine checks to see if a line has CR or LF as per RFC 5321
func validateLine(line string) error {
	if strings.ContainsAny(line, "\n\r") {
		return fmt.Errorf("smtp: A line must not contain CR or LF")
	}
	return nil
}

// validateFile checks if file exists and return it's MIME type if all ok
func validateFile(file string) (string, error) {
	if _, err := os.Stat(file); err != nil {
		return "", err
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	ct, err := getFileContentType(f)
	if err != nil {
		return "", err
	}
	return ct, nil
}

func getFileContentType(out *os.File) (string, error) {
	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err := out.Read(buffer)
	if err != nil {
		return "", err
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)
	return contentType, nil
}

func alternativeContent(msgpart, htmlText, boundary string) (string, error) {
	msgpart += fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	// text version
	plainText, err := html2text.FromString(htmlText, html2text.Options{PrettyTables: true})
	if err != nil {
		return "", err
	}
	msgpart += "\r\n" + "--" + boundary + "\r\n"
	msgpart += "Content-Type: text/plain; charset=\"UTF-8\"\r\n"
	msgpart += "Content-Transfer-Encoding: base64\r\n"
	msgpart += "\r\n" + base64.StdEncoding.EncodeToString([]byte(plainText)) + "\r\n"
	// html version
	msgpart += "\r\n" + "--" + boundary + "\r\n"
	msgpart += "Content-Type: text/html; charset=\"UTF-8\"\r\n"
	msgpart += "Content-Transfer-Encoding: base64\r\n"
	msgpart += "\r\n" + base64.StdEncoding.EncodeToString([]byte(htmlText)) + "\r\n"
	msgpart += "\r\n" + "--" + boundary + "--" + "\r\n"

	return msgpart, nil
}

// Functions below are stolen from https://github.com/emersion/go-message/blob/v0.17.0/mail/header.go
// With small modifications by me.

// generateMessageIDWithHostname generates an RFC 2822-compliant Message-Id
// based on the informational draft "Recommendations for generating Message
// IDs", it takes an hostname as argument, so that software using this library
// could use a hostname they know to be unique
func generateMessageIDWithHostname(hostname string) string {
	now := uint64(time.Now().UnixNano())

	nonceByte := make([]byte, 8)
	if _, err := rand.Read(nonceByte); err != nil {
		return ""
	}
	nonce := binary.BigEndian.Uint64(nonceByte)

	msgID := fmt.Sprintf("%s.%s@%s", base36(now), base36(nonce), hostname)
	return msgID
}

func base36(input uint64) string {
	return strings.ToUpper(strconv.FormatUint(input, 36))
}
