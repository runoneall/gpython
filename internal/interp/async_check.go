package interp

import (
	"gpython/internal/object"

	"github.com/tamnd/gopapy/v2/parser2"
)

// checkNoAsync scans an entire module before execution. Any async or
// await construct is a fatal, uncatchable interpreter error.
func checkNoAsync(file string, m *parser2.Module) error {
	if m == nil {
		return nil
	}
	var fatal error
	walkAST(m, func(node any, pos parser2.Pos) bool {
		if fatal != nil {
			return false
		}
		switch n := node.(type) {
		case *parser2.AsyncFunctionDef, *parser2.AsyncFor, *parser2.AsyncWith, *parser2.Await:
			fatal = &FatalError{
				File: file,
				Line: pos.Line,
				Col:  pos.Col,
				Msg:  "notimplementederror: async/await is not supported",
			}
			return false
		case *parser2.Comprehension:
			if n.IsAsync {
				fatal = &FatalError{
					File: file,
					Line: pos.Line,
					Col:  pos.Col,
					Msg:  "notimplementederror: async/await is not supported",
				}
				return false
			}
		}
		return true
	})
	return fatal
}

func unimplemented(_ parser2.Pos, feature string) error {
	return object.Raisef(object.NotImplementedError, "%s is not implemented", feature)
}
