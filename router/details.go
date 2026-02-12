package router

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"

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
		GroupId    string
		CurrentId  string
		OlderId    int64
		NewerId    int64
		Menu       string
		MenuLink   string
		Seen       int64
		Time       string
		Url        string
		Message    string
		Data       string
		Protocol   string
		Level      string
		Logger     string
		ServerName string
		Platform   string
		Site       string
		Frames     []parser.Frame
		Request    []request
		User       map[string]string
		Contexts   map[string]string
	}

	d := data{}
	d.Menu = "details"
	d.MenuLink = "/details/" + parts[2]
	d.GroupId = parts[2]

	// Read the latest event from the group
	var params []interface{}

	query := "SELECT d.data, d.protocol, e.message, g.url, e.id, g.level, g.logger, g.server_name, g.platform, g.site, g.seen, g.last_seen FROM `group` g LEFT JOIN event e ON g.id = e.group_id LEFT JOIN `data` d ON e.data_id = d.id WHERE g.id = ? ORDER BY e.id DESC LIMIT 1"
	params = append(params, d.GroupId)

	if len(parts) == 4 {
		d.CurrentId = parts[3]
		params = append(params, d.CurrentId)
		query = "SELECT d.data, d.protocol, e.message, g.url, e.id, g.level, g.logger, g.server_name, g.platform, g.site, g.seen, d.timestamp FROM `group` g LEFT JOIN event e ON g.id = e.group_id LEFT JOIN `data` d ON e.data_id = d.id WHERE g.id = ? AND e.id = ?"
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	err = stmt.QueryRow(params...).Scan(&d.Data, &d.Protocol, &d.Message, &d.Url, &d.CurrentId, &d.Level, &d.Logger, &d.ServerName, &d.Platform, &d.Site, &d.Seen, &d.Time)
	if err != nil {
		panic(err)
	}

	// Older event from the group
	stmt, err = db.Prepare("SELECT e.id FROM `group` g LEFT JOIN event e ON g.id = e.group_id WHERE g.id = ? AND e.id < ? ORDER BY e.id DESC LIMIT 1")
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	err = stmt.QueryRow(d.GroupId, d.CurrentId).Scan(&d.OlderId)
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}

	// Newer event from the group
	stmt, err = db.Prepare("SELECT e.id FROM `group` g LEFT JOIN event e ON g.id = e.group_id WHERE g.id = ? AND e.id > ? ORDER BY e.id ASC LIMIT 1")
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	err = stmt.QueryRow(d.GroupId, d.CurrentId).Scan(&d.NewerId)
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}

	var p parser.Packet
	err = parser.Decode(d.Data, d.Protocol, "", &p)
	if err != nil {
		panic(err)
	}

	for _, f := range p.GetFrames(d.Protocol) {
		f.Vars = template.HTML(formatVars(f.VarsRaw))
		d.Frames = append(d.Frames, f)
	}

	var http parser.Request
	if d.Protocol == "7" {
		http = p.InterfaceHttp7
	} else {
		http = p.InterfaceHttp
	}

	var httpmap map[string]interface{}
	httpjson, _ := json.Marshal(http)
	json.Unmarshal(httpjson, &httpmap)

	var keys []string
	for k := range httpmap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		v := httpmap[key]
		r := request{Name: key}
		if v == nil {
			r.Value = ""
		} else if reflect.TypeOf(v).Kind() == reflect.String {
			r.Value = v.(string)
		} else {
			var keys2 []string
			for k2 := range v.(map[string]interface{}) {
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

	d.User = detailUser(p, d.Protocol)

	d.Contexts = detailContexts(p)

	templates := template.Must(template.ParseFiles("tpl/layout.html", "tpl/details.html"))
	templates.Execute(w, d)
}

func detailUser(p parser.Packet, protocol string) map[string]string {
	result := make(map[string]string)
	var user map[string]interface{}

	if protocol == "7" {
		user = p.User
	} else {
		user = p.InterfaceUser
	}

	var keys []string
	for k := range user {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		b, _ := json.Marshal(user[key])
		result[key] = string(b)
	}
	return result
}

func detailContexts(p parser.Packet) map[string]string {
	result := make(map[string]string)
	contexts := p.Contexts

	var keys []string
	for k := range contexts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		b, _ := json.Marshal(contexts[key])
		result[key] = string(b)
	}
	return result
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
		} else if s, ok := v.(float64); ok {
			content += fmt.Sprintf("%f", s)
		} else {
			content += formatVars(v)
		}
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
		} else if s, ok := v.(float64); ok {
			content += fmt.Sprintf("%f", s)
		} else {
			content += formatVars(v)
		}
		content += `</td>`
		content += `</tr>`
	}
	return content
}
