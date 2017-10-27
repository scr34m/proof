package router

import (
	"net/http"
	"github.com/alexedwards/stack"
	"database/sql"
	"encoding/json"
	"strings"
)

func Status(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")

	db := ctx.Get("db").(*sql.DB)

	stmt, err := db.Prepare("SELECT MAX(last_seen) AS last, COUNT(*) AS c FROM `group` WHERE status = 0 AND last_seen > ?")
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	var last string
	var c int

	type data struct {
		Error bool   `json:"error"`
		Time  string `json:"time"`
		Count int    `json:"count"`
	}

	d := data{}
	d.Error = false

	err = stmt.QueryRow(parts[2]).Scan(&last, &c)
	if err != nil {
		// XXX Scan error on column index 0
	} else {
		d.Time = last
		d.Count = c
	}

	j, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
}
