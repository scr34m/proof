package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alexedwards/stack"
	_ "github.com/go-sql-driver/mysql"
	"github.com/nbari/violetear"
	"github.com/scr34m/proof/notification"
	r "github.com/scr34m/proof/router"
)

var version = "0.2"

var databaseType = flag.String("type", "sqlite", "database driver type (mysql|sqlite)")
var databaseHost = flag.String("host", "127.0.0.1", "database host")
var databasePort = flag.Int("port", 3306, "database port")
var databaseUsername = flag.String("username", "", "database user")
var databasePassword = flag.String("password", "", "database password")
var databaseName = flag.String("database", "proof.db", "database name or file")
var listen = flag.String("listen", ":2017", "Location to listen for connections")
var notificationShow = flag.Bool("notification", true, "Local notification (only OSX)")
var db *sql.DB
var notif *notification.Notification

func loggingHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx.Put("db", db)
		ctx.Put("notif", notif)
		t1 := time.Now()
		next.ServeHTTP(w, r)
		t2 := time.Now()
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
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
	log.Printf("Proof %s starting", version)

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

	router := violetear.New()
	router.AddRegex(":num", `[0-9]+`)
	router.AddRegex(":any", `*`)

	stk := stack.New(loggingHandler, recoverHandler)

	router.Handle("/", stk.Then(r.Index), "GET")
	router.Handle("/:num", stk.Then(r.Parser), "POST")
	router.Handle("/track/:num", stk.Then(r.Parser), "POST")
	router.Handle("/track/api/store", stk.Then(r.Parser), "POST")
	router.Handle("/status/:any", stk.Then(r.Status), "GET")
	router.Handle("/acknowledge/:num/:num", stk.Then(r.Acknowledge), "POST")
	router.Handle("/details/:num", stk.Then(r.Details), "GET")

	fs := http.FileServer(http.Dir("assets"))
	router.Handle("/assets/*", http.StripPrefix("/assets/", fs))

	log.Fatal(http.ListenAndServe(*listen, router))
}
