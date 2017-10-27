package main

import (
	"flag"
	"log"
	"net/http"
	"time"
	"database/sql"
	"github.com/alexedwards/stack"
	"github.com/nbari/violetear"
	r "github.com/scr34m/proof/router"
)

var version = "0.1"

var databaseSource = flag.String("database", "proof.db", "database file")
var listen = flag.String("listen", ":2017", "Location to listen for connections")
var db *sql.DB

func loggingHandler(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx.Put("db", db)
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

	db, err = sql.Open("sqlite3", *databaseSource)
	if err != nil {
		log.Fatal(err)
	}

	router := violetear.New()
	router.AddRegex(":num", `[0-9]+`)
	router.AddRegex(":any", `*`)

	stk := stack.New(loggingHandler, recoverHandler)

	router.Handle("/", stk.Then(r.Index), "GET")
	router.Handle("/track/api/store", stk.Then(r.Parser), "POST") // sentry specific
	router.Handle("/status/:any", stk.Then(r.Status), "GET")
	router.Handle("/acknowledge/:num/:num", stk.Then(r.Acknowledge), "POST")
	router.Handle("/details/:num", stk.Then(r.Details), "GET")

	fs := http.FileServer(http.Dir("assets"))
	router.Handle("/assets/*", http.StripPrefix("/assets/", fs))

	log.Fatal(http.ListenAndServe(*listen, router))
}
