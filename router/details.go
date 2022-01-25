package router

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/stack"
	"github.com/scr34m/proof/parser"
)

func Details(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")

	db := ctx.Get("db").(*sql.DB)

	type request struct {
		Name      string
		Value     string
		ValueList []request
	}

	type data struct {
		Menu       string
		MenuLink   string
		Time       string
		Message    string
		Data       string
		Level      string
		Logger     string
		ServerName string
		Platform   string
		Site       string
		Frames     []parser.Frame
		Request    []request
		User       map[string]string
	}

	d := data{}
	d.Menu = "details"
	d.MenuLink = r.URL.Path

	d.Time = time.Now().Format("2006-01-02 15:04:05")

	stmt, err := db.Prepare("SELECT d.data, e.message, g.level, g.logger, g.server_name, g.platform, g.site FROM `group` g LEFT JOIN event e ON g.id = e.group_id LEFT JOIN `data` d ON e.data_id = d.id WHERE g.id = ?")
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	err = stmt.QueryRow(parts[2]).Scan(&d.Data, &d.Message, &d.Level, &d.Logger, &d.ServerName, &d.Platform, &d.Site)
	if err != nil {
		panic(err)
	}

	m, err := parser.Decode(d.Data)
	if err != nil {
		panic(err)
	}

	trace := m["sentry.interfaces.Stacktrace"].(map[string]interface{})
	for _, v := range trace["frames"].([]interface{}) {

		_v := v.(map[string]interface{})

		f := parser.Frame{
			AbsPath:  _v["abs_path"].(string),
			Function: _v["function"].(string),
			LineNo:   _v["lineno"].(float64),
			Context:  strings.Replace(_v["context_line"].(string), " ", "\u00A0", -1),
		}

		if _v["pre_context"] != nil {
			for _, c := range _v["pre_context"].([]interface{}) {
				f.PreContext = append(f.PreContext, strings.Replace(c.(string), " ", "\u00A0", -1))
			}
		}

		if _v["post_context"] != nil {
			for _, c := range _v["post_context"].([]interface{}) {
				f.PostContext = append(f.PostContext, strings.Replace(c.(string), " ", "\u00A0", -1))
			}
		}

		if _v["vars"] != nil {
			f.Vars = template.HTML(formatVars(_v["vars"]))
		}

		d.Frames = append(d.Frames, f)
	}

	var keys []string

	http := m["sentry.interfaces.Http"].(map[string]interface{})
	for k, _ := range http {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		v := http[key]
		r := request{Name: key}
		if reflect.TypeOf(v).Kind() == reflect.String {
			r.Value = v.(string)
		} else {
			var keys2 []string
			for k2, _ := range v.(map[string]interface{}) {
				keys2 = append(keys2, k2)
			}
			sort.Strings(keys2)

			for _, key2 := range keys2 {
				b, _ := json.Marshal(v.(map[string]interface{})[key2])
				r.ValueList = append(r.ValueList, request{Name: key2, Value: string(b)})
			}
		}
		d.Request = append(d.Request, r)
	}

	d.User = make(map[string]string)
	user := m["sentry.interfaces.User"].(map[string]interface{})
	keys = nil
	for k, _ := range user {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		b, _ := json.Marshal(user[key])
		d.User[key] = string(b)
	}

	templates := template.Must(template.ParseFiles("tpl/layout.html", "tpl/details.html"))
	templates.Execute(w, d)
}

func formatVars(i interface{}) string {

	content := `<table class="ui celled striped table vars">`
	m, ok := i.(map[string]interface{})
	if ok {
		if len(m) > 0 {
			content += formatVarsMap(m)
		} else {
			return `<em>*empty*</em>`
		}
	}

	a, ok := i.([]interface{})
	if ok {
		if len(a) > 0 {
			content += formatVarsArray(a)
		} else {
			return `<em>*empty*</em>`
		}
	}

	content += `</table>`

	return content
}

func formatVarsArray(a []interface{}) string {
	content := ""
	for k, v := range a {
		content += `</tr>`
		content += `<td width="10%" nowrap><strong>`
		content += strconv.Itoa(k)
		content += `</strong></td>`
		content += `<td>`
		if s, ok := v.(string); ok {
			content += s
		} else {
			content += formatVars(v)
		}
		content += v.(string)
		content += `</td>`
		content += `</tr>`
	}
	return content
}

func formatVarsMap(m map[string]interface{}) string {
	content := ""
	for k, v := range m {
		content += `</tr>`
		content += `<td width="10%" nowrap><strong>`
		content += k
		content += `</strong></td>`
		content += `<td>`
		if s, ok := v.(string); ok {
			content += s
		} else {
			content += formatVars(v)
		}
		content += `</td>`
		content += `</tr>`
	}
	return content
}
