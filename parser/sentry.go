package parser

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/scr34m/proof/notification"
)

type Sentry struct {
	Parser
	Database *sql.DB
	Packet   Packet
	hash     string
	payload  string
}

type A []interface{}
type M map[string]interface{}

type Packet struct {
	ServerName          string `json:"server_name"`
	Project             string `json:"project"`
	Site                string `json:"site"`
	Logger              string `json:"logger"`
	Level               string `json:"level"`
	Tags                A      `json:"tags"`
	Platform            string `json:"platform"`
	Message             string `json:"message"`
	InterfaceUser       M      `json:"sentry.interfaces.User"`
	InterfaceHttp       M      `json:"sentry.interfaces.Http"`
	InterfaceException  M      `json:"sentry.interfaces.Exception"`
	InterfaceStacktrace M      `json:"sentry.interfaces.Stacktrace"`
	Timestamp           string `json:"timestamp"`
}

func (s *Sentry) Load(payload string) error {

	s.payload = payload
	s.hash = GetMD5Hash(payload)

	c, _ := base64.StdEncoding.DecodeString(s.payload)

	b := bytes.NewBufferString(string(c))

	z, err := zlib.NewReader(b)
	if err != nil {
		return err
	}
	defer z.Close()

	p, err := ioutil.ReadAll(z)
	if err != nil {
		return err
	}

	err = json.Unmarshal(p, &s.Packet)
	if err != nil {
		return err
	}

	return nil
}

func (s *Sentry) Process(notif *notification.Notification) error {
	// https://stackoverflow.com/questions/13331973/how-does-sentry-aggregate-errors

	// https://github.com/getsentry/sentry/blob/master/src/sentry/interfaces/user.py
	// ignore hash sentry.interfaces.User

	// https://github.com/getsentry/sentry/blob/master/src/sentry/interfaces/http.py
	// ignore sentry.interfaces.Http

	content := ""

	// https://github.com/getsentry/sentry/blob/master/src/sentry/interfaces/exception.py
	// sentry.interfaces.Exception
	if len(s.Packet.InterfaceException) > 0 {
		content += getContentException(s.Packet.InterfaceException)
	}

	// https://github.com/getsentry/sentry/blob/master/src/sentry/interfaces/stacktrace.py
	// sentry.interfaces.Stacktrace
	if len(s.Packet.InterfaceStacktrace) > 0 {
		content += getContentStacktrace(s.Packet.InterfaceStacktrace)
	}

	checksum := GetMD5Hash(content)

	lastSeen, err := time.Parse(time.RFC3339, s.Packet.Timestamp)
	if err != nil {
		log.Println(err)
	}

	stmt, err := s.Database.Prepare("SELECT id FROM `group` WHERE checksum = ? AND project_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	var groupId int64

	err = stmt.QueryRow(checksum, s.Packet.Project).Scan(&groupId)
	if err == nil {
		stmt, err := s.Database.Prepare("UPDATE `group` SET last_seen = ?, seen = seen + 1, status = 0 WHERE id = ?")
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.Exec(lastSeen, groupId)
		if err != nil {
			return err
		}
	} else {
		seen := 1

		view := ""
		if s.Packet.InterfaceHttp["url"] != nil {
			view = s.Packet.InterfaceHttp["url"].(string)
		}

		stmt, err := s.Database.Prepare("INSERT INTO `group` (logger, `level`, message, checksum, seen, last_seen, first_seen, project_id, `server_name`, url, site, platform, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)")
		if err != nil {
			return err
		}
		defer stmt.Close()

		res, err := stmt.Exec(s.Packet.Logger, s.Packet.Level, s.Packet.Message, checksum, seen, lastSeen, lastSeen, s.Packet.Project,
			s.Packet.ServerName, view, s.Packet.Site, s.Packet.Platform)
		if err != nil {
			return err
		}

		groupId, err = res.LastInsertId()
		if err != nil {
			return err
		}
	}

	if notif != nil {
		notif.Ping(groupId, s.Packet.Message, s.Packet.ServerName, s.Packet.Level)
	}

	stmt, err = s.Database.Prepare("INSERT INTO event (data_id, group_id, message, checksum) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(s.hash, groupId, s.Packet.Message, checksum)
	if err != nil {
		return err
	}

	// TODO store own custom format

	err = s.storeData(lastSeen)
	if err != nil {
		return err
	}

	return err
}

func (s *Sentry) storeData(lastSeen time.Time) error {
	stmt, err := s.Database.Prepare("SELECT id FROM `data` WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	var h string

	err = stmt.QueryRow(s.hash).Scan(&h)
	if err == nil { // XXX sql: no rows in result set
		return nil
	}

	stmt, err = s.Database.Prepare("INSERT INTO data (id, data, timestamp) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(s.hash, s.payload, lastSeen)
	if err != nil {
		return err
	}

	return nil
}

func getContentException(m M) string {
	output := ""

	// XXX stacktrace ignored
	output += m["type"].(string)
	output += m["value"].(string)

	/*
		Exception
			def get_hash(self, platform=None, system_frames=True):
				# optimize around the fact that some exceptions might have stacktraces
				# while others may not and we ALWAYS want stacktraces over values
				output = []
				for value in self.values:
					if not value.stacktrace:
						continue
					stack_hash = value.stacktrace.get_hash(
						platform=platform,
						system_frames=system_frames,
					)
					if stack_hash:
						output.extend(stack_hash)
						output.append(value.type)

				if not output:
					for value in self.values:
						output.extend(value.get_hash(platform=platform))

				return output
	*/
	return output
}

func getContentFrame(m M) string {
	output := ""
	if m["filename"].(string) == "[native code]" {
		log.Printf(output)
		return output
	}
	if m["module"] != nil {
		output += m["module"].(string)
	} else if m["filename"] != nil && !isUrl(m["filename"].(string)) { // XXX ignored is_caused_by
		output += m["filename"].(string)
	}
	// XXX context_line simplified
	if m["context_line"] != nil && len(m["context_line"].(string)) < 120 {
		output += m["context_line"].(string)
	} else if m["symbol"] != nil {
		output += m["symbol"].(string)
	} else if m["function"] != nil {
		output += m["function"].(string)
	} else if m["lineno"] != nil {
		output += m["lineno"].(string)
	}

	return output
}

func getContentStacktrace(m M) string {
	output := ""
	frames := m["frames"].([]interface{})
	if frames != nil {
		first := frames[0].(map[string]interface{})
		if first["function"] != nil && isUrl(first["filename"].(string)) {
			return output
		}
	}

	for _, frame := range m["frames"].([]interface{}) {
		// XXX ingored in_app value in frames
		output += getContentFrame(frame.(map[string]interface{}))
	}

	return output
}

func isUrl(s string) bool {
	if strings.Contains(s, "file:") {
		return true
	}
	if strings.Contains(s, "http:") {
		return true
	}
	if strings.Contains(s, "https:") {
		return true
	}
	return false
}

func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
