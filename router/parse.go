package router

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/alexedwards/stack"
	"github.com/go-redis/redis/v8"
	"github.com/nbari/violetear"
	"github.com/scr34m/proof/config"
	"github.com/scr34m/proof/mail"
	"github.com/scr34m/proof/notification"
	"github.com/scr34m/proof/parser"
	"github.com/scr34m/proof/shared"
)

func Parser(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
	protocol := r.Context().Value("sentry_version").(string)

	var projectId string
	num := violetear.GetParam("num", r, 0)
	if num != "" {
		projectId = num
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	queuePacket := shared.QueuePacket{
		Body:      body,
		Protocol:  protocol,
		ProjectId: projectId,
	}

	if ctx.Get("queue").(bool) {
		enqueue(ctx, queuePacket)
		return
	}

	status, err := ProcessBody(ctx.Get("db").(*sql.DB), ctx.Get("auth").(*config.AuthConfig), ctx.Get("mailer").(*mail.Mailer), queuePacket)
	if err != nil {
		panic(err)
	}

	notif := ctx.Get("notif").(*notification.Notification)
	if notif != nil && (status.IsNew || status.IsRegression) {
		notif.Ping(status.GroupId, status.Message, status.ServerName, status.Level)
	}
}

func enqueue(ctx *stack.Context, queuePacket shared.QueuePacket) {
	c := ctx.Get("ctx").(context.Context)
	redis := ctx.Get("redis").(*redis.Client)
	redisKey := ctx.Get("redisKey").(string)

	queuePacketJson, err := json.Marshal(queuePacket)
	if err != nil {
		panic(err)
	}

	err = redis.LPush(c, redisKey, queuePacketJson, 0).Err()
	if err != nil {
		panic(err)
	}
}

func ProcessBody(db *sql.DB, auth *config.AuthConfig, mailer *mail.Mailer, queuePacket shared.QueuePacket) (*parser.ProcessStatus, error) {
	s := parser.Sentry{Database: db}
	err := s.Load(queuePacket)
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
