// Command gpython runs a Python 3.14 subset using a tree-walking
// interpreter over the parser2 AST.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gpython/internal/interp"
	"gpython/internal/object"
	"gpython/module"

	"github.com/tamnd/gopapy/v2/parser2"
)

const usage = "usage: gpython [script [args...]]"

func main() {
	code := 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "runtimeerror: %s\n", strings.ToLower(fmt.Sprint(r)))
				code = 1
			}
		}()
		code = run(os.Args[1:])
	}()
	os.Exit(code)
}

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help":
			fmt.Fprintln(os.Stdout, usage)
			return 0
		}
		script := args[0]
		module.Argv = args
		i := interp.New()
		return runScript(i, script)
	}
	return runREPL()
}

func runScript(i *interp.Interpreter, script string) int {
	err := i.RunFile(script)
	if err == nil {
		return 0
	}
	return handleError(err)
}

func handleError(err error) int {
	switch e := err.(type) {
	case *interp.FatalError:
		fmt.Fprintln(os.Stderr, e.Error())
		return 1
	case *interp.SyntaxError:
		fmt.Fprintln(os.Stderr, e.Error())
		return 1
	case *object.PyError:
		if code, ok := systemExit(e); ok {
			return code
		}
		printTraceback(e)
		return 1
	default:
		fmt.Fprintf(os.Stderr, "runtimeerror: %s\n", strings.ToLower(strings.TrimSpace(err.Error())))
		return 1
	}
}

// runREPL enters an interactive read-eval-print loop. It exits on EOF
// (Ctrl-D) with code 0.
func runREPL() int {
	i := interp.New()
	i.SetScriptDir(cwdOr(""))

	env := interp.NewModuleEnv("__main__")
	env.Vars["__name__"] = object.NewStr("__main__")
	env.Vars["__file__"] = object.NewStr("<stdin>")
	env.Vars["__package__"] = object.None
	env.Vars["__doc__"] = object.None

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	var buf strings.Builder
	depth := 0

	for {
		if buf.Len() == 0 {
			fmt.Print(">>> ")
		} else {
			fmt.Print("... ")
		}
		if !scanner.Scan() {
			fmt.Println() // end the pending prompt line on EOF
			return 0
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" && depth == 0 {
			if buf.Len() > 0 {
				if err := execBlock(i, env, buf.String()); err != nil {
					fmt.Fprintln(os.Stderr, err.Error())
				}
				buf.Reset()
			}
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
		depth += netDepth(line)
		if isContinued(line, depth) {
			continue
		}
		if err := execBlock(i, env, buf.String()); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		buf.Reset()
		depth = 0
	}
}

// execBlock parses one block of source and executes it in the persistent
// env. A lone top-level expression statement has its repr printed (unless
// it is None), matching the CPython REPL.
func execBlock(i *interp.Interpreter, env *interp.Env, src string) error {
	m, err := parser2.ParseFile("<stdin>", src)
	if err != nil {
		return interp.WrapParseError(err)
	}
	body := m.Body
	if len(body) == 1 {
		if es, ok := body[0].(*parser2.ExprStmt); ok {
			v, err := i.EvalExpr(es.Value, env)
			if err != nil {
				return err
			}
			if _, isnone := v.(*object.NoneType); !isnone {
				fmt.Println(v.Repr())
			}
			return nil
		}
	}
	return i.EvalStmts(body, env)
}

// netDepth returns the change in bracket/paren/brace depth caused by a
// line, ignoring delimiters inside crude string regions.
func netDepth(line string) int {
	var d int
	inStr := false
	strQuote := byte(0)
	esc := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == strQuote {
				inStr = false
			}
			continue
		}
		if c == '\'' || c == '"' {
			inStr = true
			strQuote = c
			continue
		}
		switch c {
		case '(', '[', '{':
			d++
		case ')', ']', '}':
			d--
		}
	}
	return d
}

// isContinued reports whether the accumulator should keep reading more
// lines: an unclosed bracket group, or a simple compound-statement header
// whose last stripped line ends with ':'.
func isContinued(line string, depth int) bool {
	if depth != 0 {
		return true
	}
	trimmed := strings.TrimRight(line, " \t")
	return strings.HasSuffix(trimmed, ":")
}

func cwdOr(def string) string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return def
}

// systemExit decodes a SystemExit exception into a process exit code.
func systemExit(e *object.PyError) (int, bool) {
	inst, ok := e.Value.(*object.Instance)
	if !ok {
		return 0, false
	}
	isExit := false
	for _, c := range inst.Class.MRO {
		if c == object.SystemExit {
			isExit = true
			break
		}
	}
	if !isExit {
		return 0, false
	}
	args := object.ExceptionArgs(inst)
	if len(args) == 0 {
		return 0, true
	}
	switch code := args[0].(type) {
	case *object.NoneType:
		return 0, true
	case *object.Int:
		return int(code.V), true
	case object.Bool:
		if code {
			return 1, true
		}
		return 0, true
	case *object.Str:
		if code.V != "" {
			fmt.Fprintln(os.Stderr, code.V)
		}
		return 1, true
	default:
		fmt.Fprintln(os.Stderr, args[0].Repr())
		return 1, true
	}
}

func printTraceback(e *object.PyError) {
	fmt.Fprintln(os.Stderr, "traceback (most recent call last):")
	for idx := len(e.Trace) - 1; idx >= 0; idx-- {
		f := e.Trace[idx]
		name := f.Name
		if name == "" {
			name = "<module>"
		}
		if f.Line > 0 {
			fmt.Fprintf(os.Stderr, "  file %q, line %d, in %s\n", f.File, f.Line, name)
		} else {
			fmt.Fprintf(os.Stderr, "  file %q, in %s\n", f.File, name)
		}
	}
	fmt.Fprintln(os.Stderr, e.Error())
}
