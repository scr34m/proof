package cmd

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/alexedwards/stack"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/sessions"
	"github.com/nbari/violetear"
	"github.com/scr34m/proof/config"
	m "github.com/scr34m/proof/mail"
	"github.com/scr34m/proof/notification"
	r "github.com/scr34m/proof/router"
)

type Frontend interface {
	Start(string)
}

type frontend struct {
	ctx    context.Context
	db     *sql.DB
	notif  *notification.Notification
	auth   *config.AuthConfig
	store  *sessions.CookieStore
	mailer *m.Mailer
	redis  *redis.Client
	queue  bool
}

func NewFrontend(ctx context.Context, db *sql.DB, notif *notification.Notification, auth *config.AuthConfig, store *sessions.CookieStore, mailer *m.Mailer, redis *redis.Client, queue bool) Frontend {
	f := &frontend{
		ctx:    ctx,
		db:     db,
		notif:  notif,
		auth:   auth,
		store:  store,
		mailer: mailer,
		redis:  redis,
		queue:  queue,
	}
	return f
}

func (f *frontend) Start(listen string) {
	router := violetear.New()
	router.AddRegex(":num", `[0-9]+`)
	router.AddRegex(":any", `*`)

	stk := stack.New(f.loggingHandler, f.sessionHandler, f.recoverHandler)

	router.Handle("/", stk.Then(r.Index), "GET")
	router.Handle("/login", stk.Then(r.Login), "GET, POST")
	router.Handle("/status/:any", stk.Then(r.Status), "GET")
	router.Handle("/acknowledge/:num/:num", stk.Then(r.Acknowledge), "POST")
	router.Handle("/details/:num", stk.Then(r.Details), "GET")
	router.Handle("/details/:num/:num", stk.Then(r.Details), "GET")

	stk_basic := stack.New(f.loggingHandler, f.basicauthHandler, f.recoverHandler)

	router.Handle("/:num", stk_basic.Then(r.Parser), "POST")
	router.Handle("/track/:num", stk_basic.Then(r.Parser), "POST")
	router.Handle("/track/api/store", stk_basic.Then(r.Parser), "POST")

	fs := http.FileServer(http.Dir("assets"))
	router.Handle("/assets/*", http.StripPrefix("/assets/", fs))

	log.Fatal(http.ListenAndServe(listen, router))
}

func (f *frontend) loggingHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx.Put("db", f.db)
		ctx.Put("notif", f.notif)
		ctx.Put("auth", f.auth)
		ctx.Put("store", f.store)
		ctx.Put("mailer", f.mailer)
		ctx.Put("queue", f.queue)
		ctx.Put("ctx", f.ctx)
		ctx.Put("redis", f.redis)
		t1 := time.Now()
		next.ServeHTTP(w, r)
		t2 := time.Now()
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
	})
}

func (f *frontend) sessionHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := ctx.Get("auth").(*config.AuthConfig)
		if auth == nil {
			next.ServeHTTP(w, r)
			return
		}

		session, _ := f.store.Get(r, config.SESSION_NAME)
		err := session.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		valWithOutType := session.Values[config.COOKIE_KEY_AUTH]
		_, ok := valWithOutType.(int)

		if ok && r.URL.Path == "/login" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		} else if !ok && r.URL.Path != "/login" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func (f *frontend) basicauthHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := ctx.Get("auth").(*config.AuthConfig)
		if auth == nil {
			next.ServeHTTP(w, r)
			return
		}

		user, pass, ok := r.BasicAuth()
		if ok {
			for _, site := range auth.Site {
				if site.Enabled && user == site.Username && pass == site.Password {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), "Authentication error")
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func (f *frontend) recoverHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v", err)
				http.Error(w, http.StatusText(500), 500)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
