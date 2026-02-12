package parser

import (
	"fmt"
	"strings"
)

/**
 * https://stackoverflow.com/questions/13331973/how-does-sentry-aggregate-errors
 * https://github.com/getsentry/sentry/blob/6.4.4/src/sentry/interfaces.py
 * https://github.com/getsentry/sentry/blob/6.4.4/src/sentry/data/samples/python.json
 */

func (s *Sentry) GetChecksum() string {
	if s.protocol == "7" {
		return getChecksum7(s.Packet)
	} else {
		return getChecksum6(s.Packet)
	}
}

func getChecksum6(packet Packet) string {
	if len(packet.InterfaceStacktrace.Frames) > 0 {
		content := getContentStacktrace(packet.InterfaceStacktrace.Frames)
		if packet.InterfaceException != nil {
			content += packet.InterfaceException["type"].(string)
		}
		return GetMD5Hash(content)
	}

	if packet.InterfaceException != nil {
		content := ""
		content += packet.InterfaceException["type"].(string)
		content += packet.InterfaceException["value"].(string)
		return GetMD5Hash(content)
	}

	return GetMD5Hash(packet.Message)
}

func getChecksum7(packet Packet) string {
	content := getContentStacktrace(packet.InterfaceException7.Values[0].Stacktrace.Frames)
	content += packet.InterfaceException7.Values[0].Type
	return GetMD5Hash(content)
}

func getContentStacktrace(sframes []StackFrame) string {
	output := ""
	for _, frame := range sframes {
		output += getContentFrame(frame)
	}
	return output
}

func getContentFrame(sf StackFrame) string {
	output := ""
	if sf.Module != "" {
		output += sf.Module
	} else if sf.Filename != "" && !isUrl(sf.Filename) {
		output += sf.Filename
	}
	if sf.ContextLine != "" {
		output += sf.ContextLine
	} else if sf.Function != "" {
		output += sf.Function
	} else if sf.LineNo > 0 {
		output += fmt.Sprintf("%s", sf.LineNo)
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
