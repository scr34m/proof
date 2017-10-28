package notification

import (
	"time"
	"github.com/scr34m/gosx-notifier"
	"fmt"
)

type Notification struct {
	url      string
	duration time.Duration
	last     time.Time
	timer    *time.Timer
	Message  string
	Id       int64
	Timeout  int
}

func NewNotification(url string, interval time.Duration) *Notification {
	n := new(Notification)
	n.url = url
	n.duration = time.Millisecond * interval
	n.timer = nil
	n.Timeout = 10
	return n
}

func (n *Notification) Ping(id int64, message string) {
	n.Message = message
	n.Id = id

	if n.timer != nil {
		n.timer.Stop()
	}
	n.timer = time.AfterFunc(n.duration, n.timeout)
}

func (n *Notification) timeout() {
	n.timer = nil

	note := gosxnotifier.NewNotification(n.Message)
	note.Title = "file"
	note.Subtitle = "severity"
	note.Sound = gosxnotifier.Basso // gosxnotifier.Default
	note.Group = "proof"
	note.Remove = "proof"
	note.Sender = "com.apple.Safari"
	note.Link = fmt.Sprintf("http://%s/details/%d", n.url, n.Id)
	note.Timeout = n.Timeout
	note.Push()
}
