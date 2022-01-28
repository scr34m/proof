package router

import (
	"context"
	"database/sql"
	"io/ioutil"
	"net/http"

	"github.com/alexedwards/stack"
	"github.com/go-redis/redis/v8"
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

	if ctx.Get("queue").(bool) {
		c := ctx.Get("ctx").(context.Context)
		redis := ctx.Get("redis").(*redis.Client)
		redisKey := ctx.Get("redisKey").(string)

		err := redis.LPush(c, redisKey, body, 0).Err()
		if err != nil {
			panic(err)
		}
		return
	}

	status, err := ProcessBody(ctx.Get("db").(*sql.DB), ctx.Get("auth").(*config.AuthConfig), ctx.Get("mailer").(*mail.Mailer), string(body))
	if err != nil {
		panic(err)
	}

	notif := ctx.Get("notif").(*notification.Notification)
	if notif != nil && (status.IsNew || status.IsRegression) {
		notif.Ping(status.GroupId, status.Message, status.ServerName, status.Level)
	}
}

func ProcessBody(db *sql.DB, auth *config.AuthConfig, mailer *mail.Mailer, body string) (*parser.ProcessStatus, error) {
	s := parser.Sentry{Database: db}
	err := s.Load(body)
	if err != nil {
		return nil, err
	}

	status, err := s.Process()
	if err != nil {
		return nil, err
	}

	if mailer != nil && (status.IsNew || status.IsRegression) {
		var recipients []string
		for _, user := range auth.User {
			if user.Enabled {
				recipients = append(recipients, user.Email)
			}
		}
		mailer.Event(recipients, status)
	}

	return status, nil
}
