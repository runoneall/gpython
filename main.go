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
//
// Multi-line compound statements (def, class, if, for, try, …) are
// accumulated until the indentation stack unwinds: a line whose indent is
// not deeper than the enclosing block's indent ends the block. An empty
// line also ends the current block.
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
	var indentStack []int // 缩进栈：栈顶是当前块的起始缩进级别

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
		isBlank := strings.TrimSpace(line) == ""

		cont := shouldContinue(line, depth, indentStack, buf.Len() > 0, isBlank)

		if buf.Len() > 0 && !cont {
			// 先执行已累积的完整块（不含当前行）
			if err := execBlock(i, env, buf.String()); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			buf.Reset()
			depth = 0
			indentStack = nil
		}

		if !cont {
			// 当前行作为新块的开始（若非空）
			if !isBlank {
				buf.WriteString(line)
				depth += netDepth(line)
				trimmed := strings.TrimRight(line, " \t")
				if strings.HasSuffix(trimmed, ":") {
					indentStack = append(indentStack, indentLevel(line))
					continue
				}
				if depth == 0 {
					// 单行语句，立即执行
					if err := execBlock(i, env, buf.String()); err != nil {
						fmt.Fprintln(os.Stderr, err.Error())
					}
					buf.Reset()
					depth = 0
					indentStack = nil
				}
				// depth != 0：跨行括号表达式，继续读下一行
			}
			continue
		}

		// 继续读，累积当前行
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
		depth += netDepth(line)

		// 更新缩进栈：弹出 curIndent <= 栈顶的已结束块
		curIndent := indentLevel(line)
		for len(indentStack) > 0 && curIndent <= indentStack[len(indentStack)-1] {
			indentStack = indentStack[:len(indentStack)-1]
		}
		if strings.HasSuffix(strings.TrimRight(line, " \t"), ":") {
			indentStack = append(indentStack, curIndent)
		}

		// 如果括号已闭合且无未结束的缩进块，且不是装饰器行，
		// 立即执行（装饰器行需等待随后的 def/class）。
		if depth == 0 && len(indentStack) == 0 &&
			!strings.HasPrefix(strings.TrimLeft(line, " \t"), "@") {
			if err := execBlock(i, env, buf.String()); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			buf.Reset()
			indentStack = nil
		}
	}
}

// indentLevel 返回行首的缩进级别（空格数，制表符按 4 个空格计）。
func indentLevel(line string) int {
	n := 0
	for _, c := range line {
		if c == ' ' {
			n++
		} else if c == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}

// shouldContinue 报告是否应继续读取下一行而非执行当前已累积的块。
//
// 继续读的情况：
//   - 以 '@' 开头（装饰器，后面必须跟 def/class）；
//   - depth != 0：括号/方括号/花括号未闭合；
//   - 当前行缩进严格大于缩进栈栈顶（仍在块内）；
//   - 栈为空且当前行以 ':' 结尾（新的复合语句头，期望缩进体）。
//
// 空行在已有缓冲内容或栈非空时由调用者负责先执行块（返回 false）。
//
// 返回 false 时，调用者应先执行 buf（不含当前行），再将当前行作为新块处理。
func shouldContinue(line string, depth int, indentStack []int, hasBuf bool, isBlank bool) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "@") {
		return true
	}
	if depth != 0 {
		return true
	}
	if isBlank {
		return hasBuf || len(indentStack) > 0
	}
	curIndent := indentLevel(line)

	// 模拟弹出：找到 curIndent 应该所在的栈位置
	top := len(indentStack)
	for top > 0 && curIndent <= indentStack[top-1] {
		top--
	}

	if top == 0 {
		// 栈为空（所有块都结束）：仅以 ':' 结尾时继续读
		trimmed := strings.TrimRight(line, " \t")
		return strings.HasSuffix(trimmed, ":")
	}

	// 栈非空，curIndent > 栈顶，继续读
	return true
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
