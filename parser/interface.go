package parser

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io/ioutil"
)

type Parser interface {
	Load(string) error
	Process() (ProcessStatus, error)
}

type ProcessStatus struct {
	GroupId      int64
	Message      string
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
	Vars        template.HTML
}

func Decode(payload string) (map[string]interface{}, error) {
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
