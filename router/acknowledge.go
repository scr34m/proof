package router

import (
	"net/http"
	"github.com/alexedwards/stack"
	"database/sql"
	"encoding/json"
	"strings"
)

func Acknowledge(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")

	var status int
	if parts[3] == "1" {
		status = 1
	} else {
		status = 0
	}

	db := ctx.Get("db").(*sql.DB)

	stmt, err := db.Prepare("UPDATE `group` SET status = ? WHERE id = ?")
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(status, parts[2])
	if err != nil {
		panic(err)
	}

	type data struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	d := data{}
	d.Error = false
	d.Message = "ok"

	j, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
}
