package cmd

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/alexedwards/stack"
	"github.com/gorilla/sessions"
	"github.com/nbari/violetear"
	"github.com/scr34m/proof/config"
	m "github.com/scr34m/proof/mail"
	"github.com/scr34m/proof/notification"
	r "github.com/scr34m/proof/router"
)

type Serve interface {
	Start(listen string)
}

type serve struct {
	db     *sql.DB
	notif  *notification.Notification
	auth   *config.AuthConfig
	store  *sessions.CookieStore
	mailer *m.Mailer
}

func NewServe(db *sql.DB, notif *notification.Notification, auth *config.AuthConfig, store *sessions.CookieStore, mailer *m.Mailer) Serve {
	s := &serve{
		db:     db,
		notif:  notif,
		auth:   auth,
		store:  store,
		mailer: mailer,
	}
	return s
}

func (s *serve) Start(listen string) {
	router := violetear.New()
	router.AddRegex(":num", `[0-9]+`)
	router.AddRegex(":any", `*`)

	stk := stack.New(s.loggingHandler, s.sessionHandler, s.recoverHandler)

	router.Handle("/", stk.Then(r.Index), "GET")
	router.Handle("/login", stk.Then(r.Login), "GET, POST")
	router.Handle("/status/:any", stk.Then(r.Status), "GET")
	router.Handle("/acknowledge/:num/:num", stk.Then(r.Acknowledge), "POST")
	router.Handle("/details/:num", stk.Then(r.Details), "GET")
	router.Handle("/details/:num/:num", stk.Then(r.Details), "GET")

	stk_basic := stack.New(s.loggingHandler, s.basicauthHandler, s.recoverHandler)

	router.Handle("/:num", stk_basic.Then(r.Parser), "POST")
	router.Handle("/track/:num", stk_basic.Then(r.Parser), "POST")
	router.Handle("/track/api/store", stk_basic.Then(r.Parser), "POST")

	fs := http.FileServer(http.Dir("assets"))
	router.Handle("/assets/*", http.StripPrefix("/assets/", fs))

	log.Fatal(http.ListenAndServe(listen, router))
}

func (s *serve) loggingHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx.Put("db", s.db)
		ctx.Put("notif", s.notif)
		ctx.Put("auth", s.auth)
		ctx.Put("store", s.store)
		ctx.Put("mailer", s.mailer)
		t1 := time.Now()
		next.ServeHTTP(w, r)
		t2 := time.Now()
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
	})
}

func (s *serve) sessionHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := ctx.Get("auth").(*config.AuthConfig)
		if auth == nil {
			next.ServeHTTP(w, r)
			return
		}

		session, _ := s.store.Get(r, config.SESSION_NAME)
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

func (s *serve) basicauthHandler(ctx *stack.Context, next http.Handler) http.Handler {
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

func (s *serve) recoverHandler(ctx *stack.Context, next http.Handler) http.Handler {
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
