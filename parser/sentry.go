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

/**
 * https://stackoverflow.com/questions/13331973/how-does-sentry-aggregate-errors
 * https://github.com/getsentry/sentry/blob/6.4.4/src/sentry/interfaces.py
 * https://github.com/getsentry/sentry/blob/6.4.4/src/sentry/data/samples/python.json
 */
func (s *Sentry) GetChecksum() string {
	// ignore sentry.interfaces.User
	// ignore sentry.interfaces.Http

	/*
	   class Interface(object):

	   def get_composite_hash(self, interfaces):
	   	return self.get_hash()

	   def get_hash(self):
	   	return []
	*/

	// sentry.interfaces.Stacktrace
	/*
		class Stacktrace(Interface):
		    >>> {
		    >>>     "frames": [{
		    >>>         "abs_path": "/real/file/name.py"
		    >>>         "filename": "file/name.py",
		    >>>         "function": "myfunction",
		    >>>         "vars": {
		    >>>             "key": "value"
		    >>>         },
		    >>>         "pre_context": [
		    >>>             "line1",
		    >>>             "line2"
		    >>>         ],
		    >>>         "context_line": "line3",
		    >>>         "lineno": 3,
		    >>>         "in_app": true,
		    >>>         "post_context": [
		    >>>             "line4",
		    >>>             "line5"
		    >>>         ],
		    >>>     }],
		    >>>     "frames_omitted": [13, 56]
		    >>> }

		    def get_composite_hash(self, interfaces):
		        output = self.get_hash()
		        if 'sentry.interfaces.Exception' in interfaces:
		            exc = interfaces['sentry.interfaces.Exception'][0]
		            if exc.type:
		                output.append(exc.type)
		            elif not output:
		                output = exc.get_hash()
		        return output

		    def get_hash(self):
		        output = []
		        for frame in self.frames:
		            output.extend(frame.get_hash())
		        return output
	*/
	/*
		class Frame(object):

		def get_hash(self):
			output = []
			if self.module:
				output.append(self.module)
			elif self.filename and not self.is_url():
				output.append(self.filename)

			if self.context_line is not None:
				output.append(self.context_line)
			elif not output:
				# If we were unable to achieve any context at this point
				# (likely due to a bad JavaScript error) we should just
				# bail on recording this frame
				return output
			elif self.function:
				output.append(self.function)
			elif self.lineno is not None:
				output.append(self.lineno)
			return output
	*/

	// sentry.interfaces.Exception
	/*
		class SingleException(Interface):

			>>>  {
			>>>     "type": "ValueError",
			>>>     "value": "My exception value",
			>>>     "module": "__builtins__"
			>>>     "stacktrace": {
			>>>         # see sentry.interfaces.Stacktrace
			>>>     }
			>>> }

		def get_hash(self):
			output = None
			if self.stacktrace:
				output = self.stacktrace.get_hash()
				if output and self.type:
					output.append(self.type)
			if not output:
				output = filter(bool, [self.type, self.value])
			return output
	*/
	if s.Packet.InterfaceStacktrace != nil {
		content := getContentStacktrace(s.Packet.InterfaceStacktrace)
		if s.Packet.InterfaceException != nil {
			content += s.Packet.InterfaceException["type"].(string)
		}
		return GetMD5Hash(content)
	}

	if s.Packet.InterfaceException != nil {
		content := ""
		content += s.Packet.InterfaceException["type"].(string)
		content += s.Packet.InterfaceException["value"].(string)
		return GetMD5Hash(content)
	}

	return GetMD5Hash(s.Packet.Message)
}

func (s *Sentry) Process() (*ProcessStatus, error) {

	checksum := s.GetChecksum()

	lastSeen, err := time.Parse(time.RFC3339, s.Packet.Timestamp)
	if err != nil {
		log.Println(err)
	}

	stmt, err := s.Database.Prepare("SELECT id, status FROM `group` WHERE checksum = ? AND project_id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var groupId int64
	var status int64
	var new bool
	var regression bool

	view := ""
	if s.Packet.InterfaceHttp["url"] != nil {
		view = s.Packet.InterfaceHttp["url"].(string)
	}

	err = stmt.QueryRow(checksum, s.Packet.Project).Scan(&groupId, &status)
	if err == nil {
		new = false

		if status != 0 {
			regression = true
		}

		stmt, err := s.Database.Prepare("UPDATE `group` SET last_seen = ?, seen = seen + 1, status = 0, logger = ?, `level` = ?, message = ?, project_id = ?, `server_name` = ?, url = ?, site = ?, platform = ? WHERE id = ?")
		if err != nil {
			return nil, err
		}
		defer stmt.Close()

		_, err = stmt.Exec(lastSeen, s.Packet.Logger, s.Packet.Level, s.Packet.Message, s.Packet.Project, s.Packet.ServerName, view, s.Packet.Site, s.Packet.Platform, groupId)
		if err != nil {
			return nil, err
		}
	} else {
		seen := 1
		new = true
		regression = false

		stmt, err := s.Database.Prepare("INSERT INTO `group` (logger, `level`, message, checksum, seen, last_seen, first_seen, project_id, `server_name`, url, site, platform, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)")
		if err != nil {
			return nil, err
		}
		defer stmt.Close()

		res, err := stmt.Exec(s.Packet.Logger, s.Packet.Level, s.Packet.Message, checksum, seen, lastSeen, lastSeen, s.Packet.Project,
			s.Packet.ServerName, view, s.Packet.Site, s.Packet.Platform)
		if err != nil {
			return nil, err
		}

		groupId, err = res.LastInsertId()
		if err != nil {
			return nil, err
		}
	}

	stmt, err = s.Database.Prepare("INSERT INTO event (data_id, group_id, message, checksum) VALUES (?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}

	_, err = stmt.Exec(s.hash, groupId, s.Packet.Message, checksum)
	if err != nil {
		return nil, err
	}

	err = s.storeData(lastSeen)
	if err != nil {
		return nil, err
	}

	// Decode payload to get stacktrace
	m, err := Decode(s.payload)
	if err != nil {
		return nil, err
	}

	var frames []Frame
	trace := m["sentry.interfaces.Stacktrace"].(map[string]interface{})
	for _, v := range trace["frames"].([]interface{}) {
		_v := v.(map[string]interface{})
		f := Frame{
			AbsPath:  _v["abs_path"].(string),
			Function: _v["function"].(string),
			LineNo:   _v["lineno"].(float64),
			Context:  strings.Replace(_v["context_line"].(string), " ", "\u00A0", -1),
		}
		frames = append(frames, f)
	}

	ps := &ProcessStatus{GroupId: groupId, Message: s.Packet.Message, ServerName: s.Packet.ServerName, Site: s.Packet.Site, Level: s.Packet.Level, IsNew: new, IsRegression: regression, Frames: frames}
	return ps, nil
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

func getContentStacktrace(m M) string {
	output := ""
	for _, frame := range m["frames"].([]interface{}) {
		output += getContentFrame(frame.(map[string]interface{}))
	}
	return output
}

func getContentFrame(m M) string {
	output := ""
	if m["module"] != nil {
		output += m["module"].(string)
	} else if m["filename"] != nil && !isUrl(m["filename"].(string)) {
		output += m["filename"].(string)
	}
	if m["context_line"] != nil && m["context_line"].(string) != "" {
		output += m["context_line"].(string)
	} else if m["function"] != nil {
		output += m["function"].(string)
	} else if m["lineno"] != nil {
		output += m["lineno"].(string)
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
