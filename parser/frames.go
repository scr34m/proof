package parser

import (
	"strings"
)

func (s *Sentry) GetFrames() []Frame {
	if s.protocol == "7" {
		return GetFramesRaw(s.Packet.InterfaceException7.Values[0].Stacktrace.Frames)
	} else {
		return GetFramesRaw(s.Packet.InterfaceStacktrace.Frames)
	}
}

func (p *Packet) GetFrames(protocol string) []Frame {
	if protocol == "7" {
		return GetFramesRaw(p.InterfaceException7.Values[0].Stacktrace.Frames)
	} else {
		return GetFramesRaw(p.InterfaceStacktrace.Frames)
	}
}

func GetFramesRaw(sframes []StackFrame) []Frame {
	var frames []Frame
	for _, f := range sframes {
		frames = append(frames, getFrame(f))
	}
	return frames
}

func getFrame(sf StackFrame) Frame {
	f := Frame{
		AbsPath:  sf.AbsPath,
		Function: sf.Function,
		LineNo:   sf.LineNo,
		Context:  strings.Replace(sf.ContextLine, " ", "\u00A0", -1),
	}

	for _, s := range sf.PreContext {
		f.PreContext = append(f.PreContext, strings.Replace(s, " ", "\u00A0", -1))
	}

	for _, s := range sf.PostContext {
		f.PostContext = append(f.PostContext, strings.Replace(s, " ", "\u00A0", -1))
	}

	f.VarsRaw = sf.Vars

	return f
}
