package router

import (
	"html/template"
	"net/http"

	"github.com/alexedwards/stack"
	"github.com/gorilla/sessions"
	"github.com/scr34m/proof/config"
)

func Login(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		auth := ctx.Get("auth").(*config.AuthConfig)
		store := ctx.Get("store").(*sessions.CookieStore)

		err := r.ParseForm()
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		for idx, user := range auth.User {
			if user.Enabled && email == user.Email && password == user.Password {
				session, _ := store.Get(r, config.SESSION_NAME)
				session.Values[config.COOKIE_KEY_AUTH] = idx
				err := session.Save(r, w)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}

		http.Redirect(w, r, "/login?error=true", http.StatusFound)
		return
	}

	templates := template.Must(template.ParseFiles("tpl/login.html"))
	templates.Execute(w, nil)
}
