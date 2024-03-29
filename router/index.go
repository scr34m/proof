package router

import (
	"database/sql"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/stack"
)

func Index(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {

	db := ctx.Get("db").(*sql.DB)

	rows, err := db.Query("SELECT id, seen, url, message, last_seen, site, server_name, project_id FROM `group` WHERE status = 0 ORDER BY last_seen DESC")
	if err != nil {
		panic(err)
	}

	type event struct {
		Id                int
		Seen              int
		Url               string
		Message           string
		UrlOrMessageShort string
		LastSeen          string
		Site              string
		ServerName        string
		SiteOrServerName  string
		Project           string
	}

	var events []event

	for rows.Next() {
		event := event{}
		err = rows.Scan(&event.Id, &event.Seen, &event.Url, &event.Message, &event.LastSeen, &event.Site, &event.ServerName, &event.Project)
		if err != nil {
			panic(err)
		}

		if event.Url != "" {
			event.UrlOrMessageShort = event.Url
		} else {
			index := strings.Index(event.Message, "\n")
			if index != -1 {
				event.UrlOrMessageShort = event.Message[:index]
			} else {
				event.UrlOrMessageShort = event.Message
			}
		}

		if event.Site != "" {
			event.SiteOrServerName = event.Site
		} else {
			event.SiteOrServerName = event.ServerName
		}

		events = append(events, event)
	}

	data := struct {
		Menu     string
		MenuLink string

		Time   string
		Events []event
	}{
		Menu:     "index",
		MenuLink: "/",
		Time:     time.Now().Format("2006-01-02 15:04:05"),
		Events:   events,
	}
	templates := template.Must(template.ParseFiles("tpl/layout.html", "tpl/index.html"))
	templates.Execute(w, data)
}
