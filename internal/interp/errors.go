package interp

import (
	"fmt"
	"strings"

	"gpython/internal/object"

	"github.com/tamnd/gopapy/v2/parser2"
)

// FatalError is an uncatchable interpreter diagnostic. async/await is
// the primary user of this type: it must bypass every try/except.
type FatalError struct {
	File string
	Line int
	Col  int
	Msg  string
}

func (e *FatalError) Error() string {
	if e.File == "" {
		return e.Msg
	}
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Col, e.Msg)
}

// SyntaxError reports a parse or context error. Raw is used when the
// parser already produced a fully formatted message.
type SyntaxError struct {
	File string
	Line int
	Col  int
	Msg  string
	Raw  string
}

func (e *SyntaxError) Error() string {
	if e.Raw != "" {
		return e.Raw
	}
	if e.File == "" {
		return e.Msg
	}
	if e.Line <= 0 {
		return e.File + ": " + e.Msg
	}
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Col, e.Msg)
}

// wrapParseError lowercases the message portion of a parser2 error
// while preserving the file name and position prefix.
func wrapParseError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if idx := strings.LastIndex(msg, ": "); idx >= 0 {
		msg = msg[:idx+2] + strings.ToLower(msg[idx+2:])
	}
	return &SyntaxError{Raw: msg}
}

// WrapParseError is the exported wrapper around wrapParseError, used by
// the CLI to surface parser errors.
func WrapParseError(err error) error { return wrapParseError(err) }

func syntaxAt(file string, pos parser2.Pos, msg string) error {
	return &SyntaxError{File: file, Line: pos.Line, Col: pos.Col, Msg: msg}
}

// Control-flow signals. They are ordinary errors so that every
// evaluator function can propagate them without extra return values.
type returnSignal struct {
	value object.Object
	pos   parser2.Pos
}

func (s *returnSignal) Error() string { return "return" }

type breakSignal struct{ pos parser2.Pos }

func (s *breakSignal) Error() string { return "break" }

type continueSignal struct{ pos parser2.Pos }

func (s *continueSignal) Error() string { return "continue" }

func isLoopSignal(err error) (*breakSignal, *continueSignal) {
	switch e := err.(type) {
	case *breakSignal:
		return e, nil
	case *continueSignal:
		return nil, e
	}
	return nil, nil
}

func isPythonError(err error) bool {
	_, ok := err.(*object.PyError)
	return ok
}

func isSyntaxOrFatal(err error) bool {
	switch err.(type) {
	case *SyntaxError, *FatalError:
		return true
	}
	return false
}
