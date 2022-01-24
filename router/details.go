package router

import (
	"bytes"
	"compress/zlib"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/stack"
)

func Details(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")

	db := ctx.Get("db").(*sql.DB)

	type frame struct {
		AbsPath     string
		Function    string
		LineNo      float64
		PreContext  []string
		Context     string
		PostContext []string
		Vars        template.HTML
	}

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
		Frames     []frame
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

	m, err := decode(d.Data)
	if err != nil {
		panic(err)
	}

	trace := m["sentry.interfaces.Stacktrace"].(map[string]interface{})
	for _, v := range trace["frames"].([]interface{}) {

		_v := v.(map[string]interface{})

		f := frame{
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

	http := m["sentry.interfaces.Http"].(map[string]interface{})
	for k, v := range http {
		r := request{Name: k}
		if reflect.TypeOf(v).Kind() == reflect.String {
			r.Value = v.(string)
		} else {
			for k2, v2 := range v.(map[string]interface{}) {
				b, _ := json.Marshal(v2)
				r.ValueList = append(r.ValueList, request{Name: k2, Value: string(b)})
			}
		}
		d.Request = append(d.Request, r)
	}

	d.User = make(map[string]string)
	user := m["sentry.interfaces.User"].(map[string]interface{})
	for k, v := range user {
		b, _ := json.Marshal(v)
		d.User[k] = string(b)
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

func decode(payload string) (map[string]interface{}, error) {
	c, _ := base64.StdEncoding.DecodeString(payload)

	b := bytes.NewBufferString(string(c))

	z, err := zlib.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer z.Close()

	p, err := ioutil.ReadAll(z)
	if err != nil {
		return nil, err
	}

	var x map[string]interface{}

	err = json.Unmarshal(p, &x)
	if err != nil {
		return nil, err
	}

	return x, err
}
