package object

import "github.com/tamnd/gopapy/v2/parser2"

// Caller is implemented by the interpreter. Builtin functions use it
// to invoke user callables and to implement zero-argument super().
type Caller interface {
	Call(callee Object, args []Object, kwargs map[string]Object) (Object, error)
	Super() (Object, error)
}

// Function is a user-defined function or lambda.
type Function struct {
	Name        string
	Args        *parser2.Arguments
	Defaults    []Object // evaluated defaults for the tail of Args
	KwDefaults  []Object // aligned with Args.KwOnly; nil means required
	Body        []parser2.Stmt
	ExprBody    parser2.Expr // non-nil for lambda
	Closure     any          // *interp.Env
	File        string
	DefClass    *Class
	IsGenerator bool
	IsLambda    bool
}

func (*Function) TypeName() string { return "function" }
func (f *Function) Repr() string   { return "<function " + f.Name + ">" }
func (f *Function) Str() string    { return f.Repr() }
func (*Function) Truthy() bool     { return true }

// BoundMethod is a function bound to an instance.
type BoundMethod struct {
	Fn   *Function
	Self Object
}

func (*BoundMethod) TypeName() string { return "method" }
func (m *BoundMethod) Repr() string   { return "<bound method " + m.Fn.Name + ">" }
func (m *BoundMethod) Str() string    { return m.Repr() }
func (*BoundMethod) Truthy() bool     { return true }

// Builtin is a Go-implemented function exposed to Python code.
type Builtin struct {
	Name string
	Fn   func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error)
}

func (*Builtin) TypeName() string { return "builtin_function_or_method" }
func (b *Builtin) Repr() string   { return "<built-in function " + b.Name + ">" }
func (b *Builtin) Str() string    { return b.Repr() }
func (*Builtin) Truthy() bool     { return true }

// Call invokes the builtin.
func (b *Builtin) Call(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
	return b.Fn(ctx, args, kwargs)
}

// NewBuiltin is a small convenience constructor.
func NewBuiltin(name string, fn func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error)) *Builtin {
	return &Builtin{Name: name, Fn: fn}
}
