package router

import (
	"net/http"
	"github.com/scr34m/proof/parser"
	"io/ioutil"
	"github.com/alexedwards/stack"
	"database/sql"
)

func Parser(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	s := parser.Sentry{Database: ctx.Get("db").(*sql.DB)}
	err = s.Load(string(body));
	if err != nil {
		panic(err)
	}

	s.Process()
}
