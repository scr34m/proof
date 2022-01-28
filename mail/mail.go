package mail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"strings"

	"github.com/scr34m/proof/parser"
	gomail "gopkg.in/gomail.v2"
)

type Mailer struct {
	FromEmail string

	Host       string
	Port       int
	User       string
	Password   string
	SkipVerify bool
}

func NewMailer(host string, port int, user string, password string, verify bool, from string) *Mailer {
	return &Mailer{
		Host:       host,
		Port:       port,
		User:       user,
		Password:   password,
		SkipVerify: verify,
		FromEmail:  from,
	}
}

func (m *Mailer) Event(to []string, status *parser.ProcessStatus) {
	msg := gomail.NewMessage()

	subject := "[Proof] " + status.ServerName + " - " + strings.ToUpper(status.Level) + ": " + status.Message
	if len(subject) > 80 {
		subject = subject[:80]
	}

	var event string
	if status.IsNew {
		event = "New event"
	} else {
		event = "Regression"
	}

	addresses := make([]string, len(to))
	for i, recipient := range to {
		addresses[i] = msg.FormatAddress(recipient, "")
	}

	msg.SetHeader("From", m.FromEmail)
	msg.SetHeader("To", addresses...)
	msg.SetHeader("Subject", subject)

	var body bytes.Buffer

	var stacktrace string
	lineSize := 80

	for _, frame := range status.Frames {
		line := fmt.Sprintf("File \"%s\", line %d, in %s", frame.AbsPath, int64(frame.LineNo), frame.Function)

		for i := 0; i < len(line); i += lineSize {
			if i+lineSize > len(line) {
				stacktrace += line[i:] + "\n"
			} else {
				stacktrace += line[i:i+lineSize] + "\n"
			}
		}

		line = strings.TrimSpace(frame.Context)
		for i := 0; i < len(line); i += lineSize {
			if i+lineSize > len(line) {
				stacktrace += "  " + line[i:] + "\n"
			} else {
				stacktrace += "  " + line[i:i+lineSize] + "\n"
			}
		}
	}

	t, _ := template.ParseFiles("tpl/mail/regression.html")
	t.Execute(&body, struct {
		Event      string
		DetailsUrl string
		Site       string
		Message    string
		Stacktrace string
	}{
		Event:      event,
		DetailsUrl: fmt.Sprintf("/details/%d/", status.GroupId),
		Site:       status.ServerName,
		Message:    status.Message,
		Stacktrace: stacktrace,
	})

	msg.SetBody("text/html", body.String())

	d := gomail.NewDialer(m.Host, m.Port, m.User, m.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: m.SkipVerify}

	if err := d.DialAndSend(msg); err != nil {
		panic(err)
	}
}
