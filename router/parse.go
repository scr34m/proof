package router

import (
	"database/sql"
	"io/ioutil"
	"net/http"

	"github.com/alexedwards/stack"
	"github.com/scr34m/proof/config"
	"github.com/scr34m/proof/mail"
	"github.com/scr34m/proof/notification"
	"github.com/scr34m/proof/parser"
)

func Parser(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	s := parser.Sentry{Database: ctx.Get("db").(*sql.DB)}
	err = s.Load(string(body))
	if err != nil {
		panic(err)
	}

	status, err := s.Process()
	if err != nil {
		panic(err)
	}

	notif := ctx.Get("notif").(*notification.Notification)
	if notif != nil && (status.IsNew || status.IsRegression) {
		notif.Ping(status.GroupId, status.Message, status.ServerName, status.Level)
	}

	auth := ctx.Get("auth").(*config.AuthConfig)

	mailer := ctx.Get("mailer").(*mail.Mailer)
	if mailer != nil && (status.IsNew || status.IsRegression) {
		var recipients []string
		for _, user := range auth.User {
			if user.Enabled {
				recipients = append(recipients, user.Email)
			}
		}
		mailer.Event(recipients, status)
	}
}
