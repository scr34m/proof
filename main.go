package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
	rdb "github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/scr34m/proof/cmd"
	"github.com/scr34m/proof/config"
	m "github.com/scr34m/proof/mail"
	"github.com/scr34m/proof/notification"
)

var databaseType = flag.String("database-type", "sqlite", "Database type (mysql|sqlite)")
var databaseHost = flag.String("database-host", "127.0.0.1", "Database host")
var databasePort = flag.Int("database-port", 3306, "Database port")
var databaseUser = flag.String("database-user", "", "Database user")
var databasePassword = flag.String("database-password", "", "Database password")
var databaseName = flag.String("database", "proof.db", "Database name or file")
var listen = flag.String("listen", ":2017", "Location to listen for connections")
var notificationShow = flag.Bool("notification", true, "Local notification (only OSX)")
var authMode = flag.Bool("auth", false, "Authenticated mode")
var authDatabase = flag.String("auth-database", "proof.toml", "Authentication config")
var mail = flag.Bool("mail", false, "Enable email notifications (only with authenticated mode)")
var mailHost = flag.String("mail-host", "127.0.0.1", "SMTP server host")
var mailPort = flag.Int("mail-port", 25, "SMTP server port")
var mailUser = flag.String("mail-user", "", "SMTP server user")
var mailPassword = flag.String("mail-password", "", "SMTP server password")
var mailVerify = flag.Bool("mail-verify", true, "SMTP certificate verify")
var mailFrom = flag.String("mail-from", "noreply@example.com", "E-mail from address")
var sessionKey = flag.String("sessionkey", "", "Use custom key to encrypt cookie")
var mode = flag.String("mode", "normal", "Run mode selector (normal, worker, frontend)")
var redis = flag.String("redis", "localhost:6379", "Redis host and port")
var redisPassword = flag.String("redis-password", "", "Redis password")
var redisDb = flag.Int("redis-db", 0, "Redis database id")
var redisKey = flag.String("redis-key", "proof_events", "Redis key used to store queued events")
var url = flag.String("url", "http://localhost:2017", "Frontend URL")

var db *sql.DB
var notif *notification.Notification
var auth *config.AuthConfig
var store *sessions.CookieStore
var mailer *m.Mailer
var redisCli *rdb.Client

func main() {
	log.Printf("Proof %s starting", config.VERSION)

	flag.Parse()

	var err error

	if *databaseType == "mysql" {
		var dsn = ""

		if *databaseUser != "" && *databasePassword != "" {
			dsn += *databaseUser
			dsn += ":" + *databasePassword
			dsn += "@"
		} else if *databaseUser != "" {
			dsn += *databaseUser + "@"
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
		os.Setenv("PATH", os.Getenv("PATH")+":/opt/homebrew/bin")

		notif = notification.NewNotification(*listen, 1000)
	}

	if *authMode {
		auth = &config.AuthConfig{}
		if _, err := toml.DecodeFile(*authDatabase, auth); err != nil {
			log.Fatal(err)
		}

		store = sessions.NewCookieStore([]byte(*sessionKey))

		if *mail {
			mailer = m.NewMailer(*mailHost, *mailPort, *mailUser, *mailPassword, *mailVerify, *mailFrom, *url)
		}
	}

	if *mode == "worker" || *mode == "frontend" {
		redisCli = rdb.NewClient(&rdb.Options{
			Addr:     *redis,
			Password: *redisPassword,
			DB:       *redisDb,
		})
	}

	ctx := context.Background()

	// Start in worker mode
	if *mode == "worker" {
		c := cmd.NewWorker(ctx, db, auth, mailer, redisCli, *redisKey)
		c.Start()
		return
	}

	// Start frontend with queue decision
	var queue bool
	if *mode == "frontend" {
		queue = true
	} else {
		queue = false
	}

	c := cmd.NewFrontend(ctx, db, notif, auth, store, mailer, redisCli, *redisKey, queue)
	c.Start(*listen)
}
