package main

import (
	"crypto/rand"
	"database/sql"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/alexedwards/stack"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/nbari/violetear"
	"github.com/scr34m/proof/config"
	"github.com/scr34m/proof/notification"
	r "github.com/scr34m/proof/router"
)

var databaseType = flag.String("type", "sqlite", "database driver type (mysql|sqlite)")
var databaseHost = flag.String("host", "127.0.0.1", "database host")
var databasePort = flag.Int("port", 3306, "database port")
var databaseUsername = flag.String("username", "", "database user")
var databasePassword = flag.String("password", "", "database password")
var databaseName = flag.String("database", "proof.db", "database name or file")
var listen = flag.String("listen", ":2017", "Location to listen for connections")
var notificationShow = flag.Bool("notification", true, "Local notification (only OSX)")
var authMode = flag.Bool("auth", false, "Authenticated mode")
var authDatabase = flag.String("authdatabase", "proof.toml", "Authentication config")

var db *sql.DB
var notif *notification.Notification
var auth *config.AuthConfig
var store *sessions.CookieStore

func loggingHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx.Put("db", db)
		ctx.Put("notif", notif)
		ctx.Put("auth", auth)
		ctx.Put("store", store)
		t1 := time.Now()
		next.ServeHTTP(w, r)
		t2 := time.Now()
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
	})
}

func sessionHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := ctx.Get("auth").(*config.AuthConfig)
		if auth == nil {
			next.ServeHTTP(w, r)
			return
		}

		session, _ := store.Get(r, config.SESSION_NAME)
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

func basicauthHandler(ctx *stack.Context, next http.Handler) http.Handler {
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
					break
				}
			}
		}
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), "Authentication error")
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func recoverHandler(ctx *stack.Context, next http.Handler) http.Handler {
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

func main() {
	log.Printf("Proof %s starting", config.VERSION)

	flag.Parse()

	var err error

	if *databaseType == "mysql" {
		var dsn = ""

		if *databaseUsername != "" && *databasePassword != "" {
			dsn += *databaseUsername
			dsn += ":" + *databasePassword
			dsn += "@"
		} else if *databaseUsername != "" {
			dsn += *databaseUsername + "@"
		}
		dsn += "tcp(" + *databaseHost + ":" + strconv.Itoa(*databasePort) + ")"
		dsn += "/" + *databaseName

		db, err = sql.Open("mysql", dsn)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		db, err = sql.Open("sqlite3", *databaseName)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *notificationShow {
		// XXX for terminal-notification
		os.Setenv("PATH", os.Getenv("PATH")+":/usr/local/bin")

		notif = notification.NewNotification(*listen, 1000)
	}

	if *authMode {
		auth = &config.AuthConfig{}
		if _, err := toml.DecodeFile(*authDatabase, auth); err != nil {
			log.Fatal(err)
		}

		salt := make([]byte, config.PW_SALT_BYTES)
		_, err := io.ReadFull(rand.Reader, salt)
		if err != nil {
			log.Fatal(err)
		}

		store = sessions.NewCookieStore(salt)
	}

	router := violetear.New()
	router.AddRegex(":num", `[0-9]+`)
	router.AddRegex(":any", `*`)

	stk := stack.New(loggingHandler, sessionHandler, recoverHandler)

	router.Handle("/", stk.Then(r.Index), "GET")
	router.Handle("/login", stk.Then(r.Login), "GET, POST")
	router.Handle("/status/:any", stk.Then(r.Status), "GET")
	router.Handle("/acknowledge/:num/:num", stk.Then(r.Acknowledge), "POST")
	router.Handle("/details/:num", stk.Then(r.Details), "GET")

	stk_basic := stack.New(loggingHandler, basicauthHandler, recoverHandler)

	router.Handle("/:num", stk_basic.Then(r.Parser), "POST")
	router.Handle("/track/:num", stk_basic.Then(r.Parser), "POST")
	router.Handle("/track/api/store", stk_basic.Then(r.Parser), "POST")

	fs := http.FileServer(http.Dir("assets"))
	router.Handle("/assets/*", http.StripPrefix("/assets/", fs))

	log.Fatal(http.ListenAndServe(*listen, router))
}
