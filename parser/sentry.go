package parser

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/md5"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/scr34m/proof/shared"
)

type Parser interface {
	Load(string) error
	Process() (ProcessStatus, error)
}

type ProcessStatus struct {
	GroupId      int64
	Message      string
	Site         string
	ServerName   string
	Level        string
	Frames       []Frame
	IsNew        bool
	IsRegression bool
}

type Frame struct {
	AbsPath     string
	Function    string
	LineNo      float64
	PreContext  []string
	Context     string
	PostContext []string
	VarsRaw     I
	Vars        template.HTML
}

type Sentry struct {
	Parser
	Database  *sql.DB
	Packet    Packet
	hash      string
	payload   string
	protocol  string
	projectId string
}

type I interface{}

type A []interface{}

type M map[string]interface{}

type Value struct {
	Type       string     `json:"type"`       // 7
	Value      string     `json:"value"`      // 7
	Stacktrace Stacktrace `json:"stacktrace"` // 7
	Mechanicm  M          `json:"mechanism"`  // 7
}

type Exception struct {
	Values []Value `json:"values"` // 7
}

type Request struct {
	Url         string `json:"url"`          // 4, 7
	Method      string `json:"method"`       // 4, 7
	QueryString string `json:"query_string"` // 4, 7
	Headers     M      `json:"headers"`      // 4, 7
	Env         M      `json:"env"`          // 4
}

type Stacktrace struct {
	Frames []StackFrame `json:"frames"` // 4, 7
}

type StackFrame struct {
	AbsPath     string   `json:"abs_path"`     // 4, 7
	Filename    string   `json:"filename"`     // 4, 7
	Function    string   `json:"function"`     // 4, 7
	Module      string   `json:"module"`       // 4, 7
	LineNo      float64  `json:"lineno"`       // 4, 7
	ContextLine string   `json:"context_line"` // 4, 7
	PreContext  []string `json:"pre_context"`  // 4, 7
	PostContext []string `json:"post_context"` // 4, 7
	Vars        I        `json:"vars"`         // 4, 7
}

type Packet struct {
	ServerName          string     `json:"server_name"`                  // 4, 7
	Environment         string     `json:"environment"`                  // 7
	Project             string     `json:"project"`                      // 4
	Site                string     `json:"site"`                         // 4
	Logger              string     `json:"logger"`                       // 4
	Level               string     `json:"level"`                        // 4
	Platform            string     `json:"platform"`                     // 4, 7
	Message             string     `json:"message"`                      // 4
	User                M          `json:"user"`                         // 7
	InterfaceUser       M          `json:"sentry.interfaces.User"`       // 4
	InterfaceHttp       Request    `json:"sentry.interfaces.Http"`       // 4
	InterfaceException  M          `json:"sentry.interfaces.Exception"`  // 4
	InterfaceStacktrace Stacktrace `json:"sentry.interfaces.Stacktrace"` // 4
	InterfaceHttp7      Request    `json:"request"`                      // 7
	InterfaceException7 Exception  `json:"exception"`                    // 7
	Contexts            M          `json:"contexts"`                     // 7
	Timestamp           I          `json:"timestamp"`                    // 4: string, 7: float
}

func (s *Sentry) Load(qpacket shared.QueuePacket) error {
	if qpacket.Protocol == "7" {
		s.payload = base64.StdEncoding.EncodeToString(qpacket.Body)
	} else {
		s.payload = string(qpacket.Body)
	}
	s.hash = GetMD5Hash(s.payload)
	s.protocol = qpacket.Protocol
	s.projectId = qpacket.ProjectId
	return Decode(s.payload, s.protocol, s.projectId, &s.Packet)
}

func (s *Sentry) Process() (*ProcessStatus, error) {
	checksum := s.GetChecksum()
	lastSeen := s.GetLastSeen()
	frames := s.GetFrames()

	var url string
	if s.protocol == "7" {
		url = s.Packet.InterfaceHttp7.Url
	} else {
		url = s.Packet.InterfaceHttp.Url
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

		_, err = stmt.Exec(lastSeen, s.Packet.Logger, s.Packet.Level, s.Packet.Message, s.Packet.Project, s.Packet.ServerName, url, s.Packet.Site, s.Packet.Platform, groupId)
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
			s.Packet.ServerName, url, s.Packet.Site, s.Packet.Platform)
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
	if err == nil {
		return nil
	}

	stmt, err = s.Database.Prepare("INSERT INTO data (id, data, timestamp, protocol) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(s.hash, s.payload, lastSeen, s.protocol)
	if err != nil {
		return err
	}

	return nil
}

func Decode(payload string, protocol string, projectId string, v *Packet) error {
	var err error
	var p []byte

	if protocol == "7" {
		p, err = readGzip(payload)
		_, _, body := getParts(string(p))
		p = []byte(body)
	} else {
		p, err = readZlib(payload)
	}

	if err != nil {
		return err
	}

	err = json.Unmarshal(p, v)
	if err != nil {
		return err
	}

	if protocol == "7" {
		v.Message = v.InterfaceException7.Values[0].Value
		v.Level = v.InterfaceException7.Values[0].Type
		v.Logger = v.Platform
		v.Project = projectId
	}

	return nil
}

func readGzip(payload string) ([]byte, error) {
	c, _ := base64.StdEncoding.DecodeString(payload)
	b := bytes.NewBufferString(string(c))

	z, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer z.Close()

	return ioutil.ReadAll(z)
}

func readZlib(payload string) ([]byte, error) {
	c, _ := base64.StdEncoding.DecodeString(payload)
	b := bytes.NewBufferString(string(c))

	z, err := zlib.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer z.Close()

	return ioutil.ReadAll(z)
}

func getParts(payload string) (envelope string, header string, body string) {
	lines := strings.Split(strings.TrimSpace(payload), "\n")
	return lines[0], lines[1], lines[2]
}

func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
