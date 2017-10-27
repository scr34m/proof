package router

import (
	"net/http"
	"github.com/alexedwards/stack"
	"database/sql"
	"html/template"
	"time"
)

func Index(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {

	db := ctx.Get("db").(*sql.DB)

	rows, err := db.Query("SELECT id, seen, message, last_seen, site, server_name FROM `group` WHERE status = 0 ORDER BY last_seen DESC")
	if err != nil {
		panic(err)
	}

	type event struct {
		Id         int
		Seen       int
		Message    string
		LastSeen   string
		Site       string
		ServerName string
	}

	var events []event

	for rows.Next() {
		event := event{}
		err = rows.Scan(&event.Id, &event.Seen, &event.Message, &event.LastSeen, &event.Site, &event.ServerName)
		if err != nil {
			panic(err)
		}
		events = append(events, event)
	}

	data := struct {
		Time   string
		Events []event
	}{
		Time:   time.Now().Format("2006-01-02 15:04:05"),
		Events: events,
	}
	templates := template.Must(template.ParseFiles("tpl/layout.html", "tpl/index.html"))
	templates.Execute(w, data)
}
