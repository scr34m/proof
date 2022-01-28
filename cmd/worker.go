package cmd

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/scr34m/proof/config"
	m "github.com/scr34m/proof/mail"
	"github.com/scr34m/proof/router"
)

type Worker interface {
	Start()
}

type worker struct {
	ctx      context.Context
	db       *sql.DB
	auth     *config.AuthConfig
	mailer   *m.Mailer
	redis    *redis.Client
	redisKey string
}

func NewWorker(ctx context.Context, db *sql.DB, auth *config.AuthConfig, mailer *m.Mailer, redis *redis.Client, redisKey string) Worker {
	w := &worker{
		ctx:      ctx,
		db:       db,
		auth:     auth,
		mailer:   mailer,
		redis:    redis,
		redisKey: redisKey,
	}
	return w
}

func (w *worker) Start() {
	ctx, cancel := context.WithCancel(w.ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	defer func() {
		signal.Stop(c)
		cancel()
	}()

	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := w.loop(ctx); err != nil {
		log.Panic(err)
	}
}

func (w *worker) loop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			val, _ := w.redis.BRPop(w.ctx, 1*time.Second, w.redisKey).Result()
			if val != nil {
				if val[1] != "0" {
					log.Println("Processing event")
					_, err := router.ProcessBody(w.db, w.auth, w.mailer, val[1])
					if err != nil {
						return err
					}
				}
			}
		}
	}
}
