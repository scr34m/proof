package parser

import (
	"time"
)

func (s *Sentry) GetLastSeen() time.Time {
	var lastSeen time.Time
	if s.protocol == "7" {
		sec := int64(s.Packet.Timestamp.(float64))
		nsec := int64((s.Packet.Timestamp.(float64) - float64(sec)) * 1e9)
		return time.Unix(sec, nsec)
	} else {
		lastSeen, _ = time.Parse(time.RFC3339, s.Packet.Timestamp.(string))
		return lastSeen
	}
}
