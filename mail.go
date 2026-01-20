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
	gomail "net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Necoro/html2text"
	"github.com/wneessen/go-mail"
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

// MailSend sends mail via smtp.
// Supports multiple recepients, TLS (port 465)/StartTLS(ports 25,587, any other).
// Mail should always valid (correctly encoded subject and body).
// Now there is HTML (with automatic text version generating) support.
// We can also send attachments.
func MailSend(n MailConfig) error {

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

	//if n.Hostname == "" {
	//	return fmt.Errorf("SendMail: hostname is not defined, it is needed to generate unique Message-Id (SENDING ABORTED)")
	//}

	recipients := strings.Join(n.To, ", ")

	// subjectb64 := base64.StdEncoding.EncodeToString([]byte(n.Subject))

	// if from contains not only email, but something like "Name Surname <mail@something.com>"
	// we need to be sure international chars are properly encoded

	header := make(map[string]string)
	header["Date"] = time.Now().Format(time.RFC1123Z)
	//header["Subject"] = "=?UTF-8?B?" + subjectb64 + "?="
	header["MIME-Version"] = "1.0"
	header["Message-Id"] = fmt.Sprintf("<%s>", generateMessageIDWithHostname(n.Hostname))

	if n.Sender != "" {
		header["Sender"] = n.Sender
	}
	// remove reply to - it can be problem for antispam
	//if n.ReplyTo != "" {
	//	header["Reply-To"] = n.ReplyTo
	//}
	isHTML := strings.Contains(n.Text, "<html")
	hasAttachments := len(n.Files) > 0
	hasEmbeddedImgs := len(n.EmbededHTMLImgs) > 0

	message := mail.NewMsg(mail.WithEncoding(mail.EncodingB64))
	from, err := gomail.ParseAddress(n.From)
	if err != nil {
		log.Printf("error parsing 'from' email %q, err: %v, trying to use backup method", n.From, err)
		if err := message.From(n.From); err != nil {
			return fmt.Errorf("failed to set FROM address: %s", err)
		}
	} else {
		message.FromMailAddress(from)
	}

	if err := message.To(recipients); err != nil {
		return fmt.Errorf("failed to set TO address: %s", err)
	}

	message.Subject(n.Subject)
	message.SetMessageID()
	message.SetDate()

	if isHTML {
		//message.SetBodyString(mail.TypeTextHTML, n.Text)
		plainText, err := html2text.FromString(n.Text, html2text.Options{PrettyTables: true})
		if err != nil {
			log.Printf("error converting HTML to plain text, err: %v", err)
		}
		// log.Printf("plain body: %q", plainText) // DEBUG
		// log.Printf("html text: %q", n.Text)     // DEBUG
		message.SetBodyString(mail.TypeTextPlain, plainText)
		message.AddAlternativeString(mail.TypeTextHTML, n.Text)
	} else {
		message.SetBodyString(mail.TypeTextPlain, n.Text)
	}

	if hasAttachments {
		for i := range n.Files {
			message.AttachFile(n.Files[i])
		}
	}

	if hasEmbeddedImgs {
		for i := range n.EmbededHTMLImgs {
			message.EmbedFile(n.EmbededHTMLImgs[i].NamePath)
			log.Printf("DEBUG: embedding file %q", n.EmbededHTMLImgs[i].NamePath) // DEBUG
		}
	}

	message.WriteToFile("tmp/test.msg")

	// Create a custom TLS config that skips verification
	tlsConf := &tls.Config{
		InsecureSkipVerify: true, // ⚠️ needed for our server, by IP cert is wrong
	}

	// Deliver the mails via SMTP
	client, err := mail.NewClient(n.Server,
		mail.WithTimeout(2*time.Minute), // increased timeout, attached pdf tickets were problematic
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover), mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithUsername(n.User), mail.WithPassword(n.Pass), mail.WithTLSConfig(tlsConf),
	)
	if err != nil {
		return fmt.Errorf("failed to create new mail delivery client: %s", err)
	}
	if err := client.DialAndSend(message); err != nil {
		return fmt.Errorf("failed to deliver mail: %s", err)
	}
	log.Printf("Mail successfully delivered.")

	//err = sendMail(n.Server, n.Port, auth, n.IgnoreCert, n.From, n.To, []byte(message))

	if err != nil {
		return fmt.Errorf("SendMail: error sending mail, err: %v", err)
	}

	return nil
}

func encodeRFC2047(s string) string {
	// use mail's rfc2047 to encode any string
	addr := gomail.Address{Name: s, Address: ""}
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
