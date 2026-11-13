package interp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gpython/internal/builtins"
	"gpython/internal/object"
	"gpython/module"

	"github.com/tamnd/gopapy/v2/parser2"
)

// Frame is one active Python call frame. It powers zero-argument
// super() and the lowercase traceback.
type Frame struct {
	Name       string
	File       string
	Line       int
	Env        *Env
	DefClass   *object.Class
	Self       object.Object
	IsFunction bool
	Prev       *Frame
}

// Interpreter evaluates parser2 trees.
type Interpreter struct {
	Builtins map[string]object.Object
	Modules  map[string]*object.Module
	Curr     *Frame
	Pos      parser2.Pos
	Depth    int
	MaxDepth int

	gen *object.Generator

	loopDepth int
	excStack  []object.Object

	// Context of the module currently being executed, used for
	// relative imports.
	moduleName      string
	moduleDir       string
	moduleIsPackage bool
	scriptDir       string
}

// New creates an interpreter with a fresh builtin namespace.
func New() *Interpreter {
	return &Interpreter{
		Builtins: builtins.Scope(),
		Modules:  map[string]*object.Module{},
		MaxDepth: 900,
	}
}

// RunFile parses and executes a script.
func (i *Interpreter) RunFile(filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return &FatalError{Msg: fmt.Sprintf("filenotfounderror: no such file or directory: '%s'", filename)}
	}
	m, err := parser2.ParseFile(filename, string(src))
	if err != nil {
		return wrapParseError(err)
	}
	i.scriptDir = filepath.Dir(filename)
	return i.runModule("__main__", filename, m, false)
}

// RunSource is a convenience wrapper used by tests.
func (i *Interpreter) RunSource(filename, src string) error {
	m, err := parser2.ParseFile(filename, src)
	if err != nil {
		return wrapParseError(err)
	}
	i.scriptDir = filepath.Dir(filename)
	return i.runModule("__main__", filename, m, false)
}

func (i *Interpreter) runModule(name, file string, m *parser2.Module, isPackage bool) error {
	if err := checkNoAsync(file, m); err != nil {
		return err
	}
	env := NewModuleEnv(name)
	mod := object.NewModule(name)
	mod.Dict = env.Vars
	mod.File = file
	mod.Dir = filepath.Dir(file)
	mod.IsPackage = isPackage

	env.Vars["__name__"] = object.NewStr(name)
	env.Vars["__file__"] = object.NewStr(file)
	pkg := ""
	if isPackage {
		pkg = name
	} else if idx := strings.LastIndexByte(name, '.'); idx >= 0 {
		pkg = name[:idx]
	}
	env.Vars["__package__"] = object.NewStr(pkg)
	env.Vars["__doc__"] = object.None

	i.Modules[name] = mod
	module.CacheSysModule(name, mod)

	restore := i.enterModule(name, filepath.Dir(file), isPackage)
	defer restore()

	frame := &Frame{Name: "<module>", File: file, Env: env}
	i.pushFrame(frame)
	defer i.popFrame(frame)

	err := i.EvalStmts(m.Body, env)
	if pe, ok := err.(*object.PyError); ok {
		pe.Trace = append(pe.Trace, object.TraceFrame{File: file, Line: frame.Line, Name: "<module>"})
	}
	return err
}

func (i *Interpreter) enterModule(name, dir string, isPackage bool) func() {
	oldName, oldDir, oldPkg := i.moduleName, i.moduleDir, i.moduleIsPackage
	i.moduleName, i.moduleDir, i.moduleIsPackage = name, dir, isPackage
	return func() {
		i.moduleName, i.moduleDir, i.moduleIsPackage = oldName, oldDir, oldPkg
	}
}

func (i *Interpreter) pushFrame(f *Frame) {
	f.Prev = i.Curr
	i.Curr = f
}

func (i *Interpreter) popFrame(f *Frame) {
	i.Curr = f.Prev
}

// SetScriptDir sets the directory used for resolving relative imports and
// local modules. Used by the CLI/REPL.
func (i *Interpreter) SetScriptDir(dir string) {
	i.scriptDir = dir
}

// clone creates an independent evaluator state for a generator body.
// The module cache and builtins are shared.
func (i *Interpreter) clone() *Interpreter {
	c := *i
	c.Curr = nil
	c.Pos = parser2.Pos{}
	c.gen = nil
	return &c
}

func (i *Interpreter) lookupName(env *Env, name string) (object.Object, error) {
	if v, ok := env.Get(name); ok {
		return v, nil
	}
	if v, ok := i.Builtins[name]; ok {
		return v, nil
	}
	return nil, object.Raisef(object.NameError, "name '%s' is not defined", name)
}

// EvalStmts executes a statement list in env.
func (i *Interpreter) EvalStmts(stmts []parser2.Stmt, env *Env) error {
	for _, s := range stmts {
		if i.Curr != nil {
			i.Curr.Line = nodePos(s).Line
		}
		if err := i.evalStmt(s, env); err != nil {
			return err
		}
	}
	return nil
}

// EvalExpr evaluates a single expression in env. Exposed for the REPL.
func (i *Interpreter) EvalExpr(e parser2.Expr, env *Env) (object.Object, error) {
	return i.evalExpr(e, env)
}
