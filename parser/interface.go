package parser

import "github.com/scr34m/proof/notification"

type Parser interface {
	Load(string) error
	Process(*notification.Notification) error
}
