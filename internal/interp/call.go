package interp

import (
	"math"
	"strconv"
	"strings"

	"gpython/internal/object"

	"github.com/tamnd/gopapy/v2/parser2"
)

func (i *Interpreter) evalDefaults(args *parser2.Arguments, env *Env) ([]object.Object, []object.Object, error) {
	if args == nil {
		return nil, nil, nil
	}
	defaults := make([]object.Object, len(args.Defaults))
	for idx, e := range args.Defaults {
		v, err := i.evalExpr(e, env)
		if err != nil {
			return nil, nil, err
		}
		defaults[idx] = v
	}
	kwDefaults := make([]object.Object, len(args.KwOnly))
	for idx, e := range args.KwOnlyDef {
		if e == nil {
			continue
		}
		v, err := i.evalExpr(e, env)
		if err != nil {
			return nil, nil, err
		}
		kwDefaults[idx] = v
	}
	return defaults, kwDefaults, nil
}

func (i *Interpreter) evalCall(n *parser2.Call, env *Env) (object.Object, error) {
	callee, err := i.evalExpr(n.Func, env)
	if err != nil {
		return nil, err
	}
	args := make([]object.Object, 0, len(n.Args))
	for _, ae := range n.Args {
		if star, ok := ae.(*parser2.Starred); ok {
			v, err := i.evalExpr(star.Value, env)
			if err != nil {
				return nil, err
			}
			it, err := object.Iterate(v)
			if err != nil {
				return nil, err
			}
			for {
				x, err := it.Next()
				if err == object.ErrIterationDone {
					break
				}
				if err != nil {
					return nil, err
				}
				args = append(args, x)
			}
			continue
		}
		v, err := i.evalExpr(ae, env)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	kwargs := map[string]object.Object{}
	for _, kw := range n.Keywords {
		v, err := i.evalExpr(kw.Value, env)
		if err != nil {
			return nil, err
		}
		if kw.Arg == "" {
			m, ok := v.(*object.Dict)
			if !ok {
				return nil, object.Raisef(object.TypeErrorClass, "argument after ** must be a mapping, not '%s'", v.TypeName())
			}
			for idx, key := range m.Keys {
				s, ok := key.(*object.Str)
				if !ok {
					return nil, object.Raisef(object.TypeErrorClass, "keywords must be strings")
				}
				if _, exists := kwargs[s.V]; exists {
					return nil, object.Raisef(object.TypeErrorClass, "got multiple values for keyword argument '%s'", s.V)
				}
				kwargs[s.V] = m.Values[idx]
			}
			continue
		}
		if _, exists := kwargs[kw.Arg]; exists {
			return nil, object.Raisef(object.TypeErrorClass, "got multiple values for keyword argument '%s'", kw.Arg)
		}
		kwargs[kw.Arg] = v
	}
	return i.Call(callee, args, kwargs)
}

// Call invokes any callable value.
func (i *Interpreter) Call(callee object.Object, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	switch c := callee.(type) {
	case *object.Builtin:
		return c.Call(i, args, kwargs)
	case *object.BoundMethod:
		all := make([]object.Object, 0, len(args)+1)
		all = append(all, c.Self)
		all = append(all, args...)
		return i.callFunction(c.Fn, all, kwargs)
	case *object.Function:
		return i.callFunction(c, args, kwargs)
	case *object.Class:
		return i.callClass(c, args, kwargs)
	case *object.Instance:
		// 实例若定义了 __call__，则可调用：m() 等价于 type(m).__call__(m)。
		// 将 __call__ 绑定到实例（如同方法），再递归调用，复用 BoundMethod 分支。
		callAttr, ok := c.Class.Lookup("__call__")
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "'%s' object is not callable", c.TypeName())
		}
		if fn, ok := callAttr.(*object.Function); ok {
			bound := &object.BoundMethod{Fn: fn, Self: c}
			return i.Call(bound, args, kwargs)
		}
		if cc, ok := callAttr.(object.Callable); ok {
			return cc.Call(i, args, kwargs)
		}
		return nil, object.Raisef(object.TypeErrorClass, "'%s' object is not callable", c.TypeName())
	case object.Callable:
		return c.Call(i, args, kwargs)
	}
	if callee == nil {
		return nil, object.Raisef(object.TypeErrorClass, "object is not callable")
	}
	return nil, object.Raisef(object.TypeErrorClass, "'%s' object is not callable", callee.TypeName())
}

// Super implements zero-argument super() for the current frame.
func (i *Interpreter) Super() (object.Object, error) {
	if i.Curr == nil || i.Curr.DefClass == nil || i.Curr.Self == nil {
		return nil, object.Raisef(object.RuntimeError, "super(): no arguments")
	}
	return &object.Super{Class: i.Curr.DefClass, Self: i.Curr.Self}, nil
}

func (i *Interpreter) callFunction(fn *object.Function, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if i.Depth >= i.MaxDepth {
		return nil, object.Raisef(object.RecursionError, "maximum recursion depth exceeded")
	}
	i.Depth++
	defer func() { i.Depth-- }()

	closure, _ := fn.Closure.(*Env)
	if closure == nil {
		closure = NewModuleEnv("<unknown>")
	}
	env := closure.Child(EnvFunction, fn.Name)
	if err := bindArgs(env, fn, args, kwargs); err != nil {
		return nil, err
	}
	var self object.Object
	if fn.DefClass != nil && len(args) > 0 {
		self = args[0]
	}

	frame := &Frame{
		Name:       fn.Name,
		File:       fn.File,
		Env:        env,
		DefClass:   fn.DefClass,
		Self:       self,
		IsFunction: true,
	}

	if fn.IsGenerator {
		sub := i.clone()
		sub.loopDepth = 0
		sub.pushFrame(frame)
		return object.NewGenerator(fn.Name, func(g *object.Generator) error {
			sub.gen = g
			err := sub.EvalStmts(fn.Body, env)
			if _, ok := err.(*returnSignal); ok {
				err = nil
			}
			if _, ok := err.(*breakSignal); ok {
				err = object.Raisef(object.SyntaxErrorClass, "'break' outside loop")
			}
			if _, ok := err.(*continueSignal); ok {
				err = object.Raisef(object.SyntaxErrorClass, "'continue' outside loop")
			}
			if pe, ok := err.(*object.PyError); ok {
				pe.Trace = append(pe.Trace, object.TraceFrame{File: fn.File, Line: frame.Line, Name: fn.Name})
			}
			sub.popFrame(frame)
			return err
		}), nil
	}

	i.pushFrame(frame)
	defer i.popFrame(frame)
	oldLoop := i.loopDepth
	i.loopDepth = 0
	defer func() { i.loopDepth = oldLoop }()

	if fn.IsLambda {
		v, err := i.evalExpr(fn.ExprBody, env)
		if err != nil {
			if pe, ok := err.(*object.PyError); ok {
				pe.Trace = append(pe.Trace, object.TraceFrame{File: fn.File, Line: frame.Line, Name: fn.Name})
			}
			return nil, err
		}
		return v, nil
	}

	err := i.EvalStmts(fn.Body, env)
	if rs, ok := err.(*returnSignal); ok {
		return rs.value, nil
	}
	if err != nil {
		if _, ok := err.(*breakSignal); ok {
			return nil, object.Raisef(object.SyntaxErrorClass, "'break' outside loop")
		}
		if _, ok := err.(*continueSignal); ok {
			return nil, object.Raisef(object.SyntaxErrorClass, "'continue' outside loop")
		}
		if pe, ok := err.(*object.PyError); ok {
			pe.Trace = append(pe.Trace, object.TraceFrame{File: fn.File, Line: frame.Line, Name: fn.Name})
		}
		return nil, err
	}
	return object.None, nil
}

func bindArgs(env *Env, fn *object.Function, args []object.Object, kwargs map[string]object.Object) error {
	a := fn.Args
	if a == nil {
		a = &parser2.Arguments{}
	}
	params := make([]*parser2.Arg, 0, len(a.PosOnly)+len(a.Args))
	params = append(params, a.PosOnly...)
	params = append(params, a.Args...)
	bound := make([]bool, len(params)+len(a.KwOnly))

	var varargs []object.Object
	for idx, v := range args {
		if idx < len(params) {
			env.Set(params[idx].Name, v)
			bound[idx] = true
			continue
		}
		if a.Vararg == nil {
			return object.Raisef(object.TypeErrorClass, "%s() takes %d positional argument%s but %d %s given",
				fn.Name, len(params), plural(len(params)), len(args), wasWere(len(args)))
		}
		varargs = append(varargs, v)
	}
	if a.Vararg != nil {
		env.Set(a.Vararg.Name, object.NewTuple(varargs...))
	}

	posOnlyIdx := map[string]int{}
	for idx, p := range a.PosOnly {
		posOnlyIdx[p.Name] = idx
	}
	argIdx := map[string]int{}
	for idx, p := range a.Args {
		argIdx[p.Name] = len(a.PosOnly) + idx
	}
	kwOnlyIdx := map[string]int{}
	for idx, p := range a.KwOnly {
		kwOnlyIdx[p.Name] = idx
	}

	extra := object.NewDict()
	for name, v := range kwargs {
		if _, ok := posOnlyIdx[name]; ok {
			return object.Raisef(object.TypeErrorClass, "%s() got some positional-only arguments passed as keyword arguments: '%s'", fn.Name, name)
		}
		if idx, ok := argIdx[name]; ok {
			if bound[idx] {
				return object.Raisef(object.TypeErrorClass, "%s() got multiple values for argument '%s'", fn.Name, name)
			}
			env.Set(name, v)
			bound[idx] = true
			continue
		}
		if idx, ok := kwOnlyIdx[name]; ok {
			if bound[len(params)+idx] {
				return object.Raisef(object.TypeErrorClass, "%s() got multiple values for argument '%s'", fn.Name, name)
			}
			env.Set(name, v)
			bound[len(params)+idx] = true
			continue
		}
		if a.Kwarg != nil {
			if err := extra.Set(object.NewStr(name), v); err != nil {
				return err
			}
			continue
		}
		return object.Raisef(object.TypeErrorClass, "%s() got an unexpected keyword argument '%s'", fn.Name, name)
	}

	// Positional defaults cover the tail of a.Args.
	defaultStart := len(a.Args) - len(fn.Defaults)
	var missing []string
	for idx, p := range params {
		if bound[idx] {
			continue
		}
		if idx >= len(a.PosOnly) {
			j := idx - len(a.PosOnly)
			if j >= defaultStart && j-defaultStart < len(fn.Defaults) {
				env.Set(p.Name, fn.Defaults[j-defaultStart])
				continue
			}
		}
		missing = append(missing, p.Name)
	}
	if len(missing) > 0 {
		return missingPositional(fn.Name, missing)
	}

	var missingKw []string
	for idx, p := range a.KwOnly {
		if bound[len(params)+idx] {
			continue
		}
		if idx < len(fn.KwDefaults) && fn.KwDefaults[idx] != nil {
			env.Set(p.Name, fn.KwDefaults[idx])
			continue
		}
		missingKw = append(missingKw, p.Name)
	}
	if len(missingKw) > 0 {
		if len(missingKw) == 1 {
			return object.Raisef(object.TypeErrorClass, "%s() missing 1 required keyword-only argument: '%s'", fn.Name, missingKw[0])
		}
		return object.Raisef(object.TypeErrorClass, "%s() missing %d required keyword-only arguments: %s", fn.Name, len(missingKw), quoteList(missingKw))
	}
	if a.Kwarg != nil {
		env.Set(a.Kwarg.Name, extra)
	}
	return nil
}

func missingPositional(name string, missing []string) error {
	if len(missing) == 1 {
		return object.Raisef(object.TypeErrorClass, "%s() missing 1 required positional argument: '%s'", name, missing[0])
	}
	return object.Raisef(object.TypeErrorClass, "%s() missing %d required positional arguments: %s", name, len(missing), quoteList(missing))
}

func quoteList(names []string) string {
	parts := make([]string, len(names))
	for idx, n := range names {
		parts[idx] = "'" + n + "'"
	}
	return strings.Join(parts, ", ")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func wasWere(n int) string {
	if n == 1 {
		return "was"
	}
	return "were"
}

func (i *Interpreter) callClass(c *object.Class, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if c.BuiltinKind != "" {
		return i.callBuiltinClass(c, args, kwargs)
	}
	if c == object.ObjectClass {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "object() takes no arguments")
		}
		return object.NewInstance(c), nil
	}
	var inst *object.Instance
	if c.IsException {
		inst = object.NewException(c, args...)
	} else {
		inst = object.NewInstance(c)
	}
	if init, ok := c.Lookup("__init__"); ok {
		if fn, ok := init.(*object.Function); ok {
			all := make([]object.Object, 0, len(args)+1)
			all = append(all, inst)
			all = append(all, args...)
			if _, err := i.callFunction(fn, all, kwargs); err != nil {
				return nil, err
			}
		}
	}
	return inst, nil
}

func noKwargs(name string, kwargs map[string]object.Object) error {
	for k := range kwargs {
		return object.Raisef(object.TypeErrorClass, "%s() takes no keyword arguments (got '%s')", name, k)
	}
	return nil
}

func (i *Interpreter) callBuiltinClass(c *object.Class, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	switch c.BuiltinKind {
	case "type":
		if err := noKwargs("type", kwargs); err != nil {
			return nil, err
		}
		if len(args) == 1 {
			return object.ClassOf(args[0]), nil
		}
		return nil, object.Raisef(object.NotImplementedError, "type() with 3 arguments is not supported")
	case "int":
		return builtinInt(args, kwargs)
	case "float":
		return builtinFloat(args, kwargs)
	case "bool":
		if err := noKwargs("bool", kwargs); err != nil {
			return nil, err
		}
		if len(args) == 0 {
			return object.False, nil
		}
		if len(args) == 1 {
			return object.BoolFrom(args[0].Truthy()), nil
		}
		return nil, object.Raisef(object.TypeErrorClass, "bool() takes at most 1 argument (%d given)", len(args))
	case "str":
		if err := noKwargs("str", kwargs); err != nil {
			return nil, err
		}
		if len(args) == 0 {
			return object.NewStr(""), nil
		}
		if len(args) == 1 {
			return object.NewStr(args[0].Str()), nil
		}
		return nil, object.Raisef(object.TypeErrorClass, "str() takes at most 1 argument (%d given)", len(args))
	case "list":
		if err := noKwargs("list", kwargs); err != nil {
			return nil, err
		}
		if len(args) == 0 {
			return object.NewList(), nil
		}
		if len(args) > 1 {
			return nil, object.Raisef(object.TypeErrorClass, "list() takes at most 1 argument (%d given)", len(args))
		}
		elts, err := iterableValues(args[0])
		if err != nil {
			return nil, err
		}
		return object.NewList(elts...), nil
	case "tuple":
		if err := noKwargs("tuple", kwargs); err != nil {
			return nil, err
		}
		if len(args) == 0 {
			return object.NewTuple(), nil
		}
		if len(args) > 1 {
			return nil, object.Raisef(object.TypeErrorClass, "tuple() takes at most 1 argument (%d given)", len(args))
		}
		elts, err := iterableValues(args[0])
		if err != nil {
			return nil, err
		}
		return object.NewTuple(elts...), nil
	case "set":
		if err := noKwargs("set", kwargs); err != nil {
			return nil, err
		}
		set := object.NewSet()
		if len(args) == 0 {
			return set, nil
		}
		if len(args) > 1 {
			return nil, object.Raisef(object.TypeErrorClass, "set() takes at most 1 argument (%d given)", len(args))
		}
		elts, err := iterableValues(args[0])
		if err != nil {
			return nil, err
		}
		for _, v := range elts {
			if err := set.Add(v); err != nil {
				return nil, err
			}
		}
		return set, nil
	case "dict":
		return builtinDict(args, kwargs)
	case "range":
		return builtinRange(args, kwargs)
	}
	return nil, object.Raisef(object.NotImplementedError, "constructing %s is not supported", c.Name)
}

func builtinInt(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	base := int64(10)
	baseGiven := false
	if v, ok := kwargs["base"]; ok {
		b, ok := toInt(v)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "int() base must be an integer")
		}
		base = b
		baseGiven = true
	}
	for k := range kwargs {
		if k != "base" {
			return nil, object.Raisef(object.TypeErrorClass, "int() got an unexpected keyword argument '%s'", k)
		}
	}
	if len(args) == 0 {
		return object.NewInt(0), nil
	}
	if len(args) > 2 {
		return nil, object.Raisef(object.TypeErrorClass, "int() takes at most 2 arguments (%d given)", len(args))
	}
	if len(args) == 2 {
		b, ok := toInt(args[1])
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "int() base must be an integer")
		}
		base = b
		baseGiven = true
	}
	switch x := args[0].(type) {
	case *object.Int:
		if baseGiven {
			return nil, object.Raisef(object.TypeErrorClass, "int() can't convert non-string with explicit base")
		}
		return object.NewInt(x.V), nil
	case object.Bool:
		if baseGiven {
			return nil, object.Raisef(object.TypeErrorClass, "int() can't convert non-string with explicit base")
		}
		if x {
			return object.NewInt(1), nil
		}
		return object.NewInt(0), nil
	case *object.Float:
		if baseGiven {
			return nil, object.Raisef(object.TypeErrorClass, "int() can't convert non-string with explicit base")
		}
		if math.IsInf(x.V, 0) || math.IsNaN(x.V) {
			return nil, object.Raisef(object.ValueErrorClass, "cannot convert float infinity to integer")
		}
		return object.NewInt(int64(x.V)), nil
	case *object.Str:
		s := strings.TrimSpace(x.V)
		v, err := strconv.ParseInt(s, int(base), 64)
		if err != nil {
			return nil, object.Raisef(object.ValueErrorClass, "invalid literal for int() with base %d: '%s'", base, x.V)
		}
		return object.NewInt(v), nil
	}
	return nil, object.Raisef(object.TypeErrorClass, "int() argument must be a string or a number, not '%s'", args[0].TypeName())
}

func builtinFloat(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if err := noKwargs("float", kwargs); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return object.NewFloat(0), nil
	}
	if len(args) > 1 {
		return nil, object.Raisef(object.TypeErrorClass, "float() takes at most 1 argument (%d given)", len(args))
	}
	switch x := args[0].(type) {
	case *object.Float:
		return object.NewFloat(x.V), nil
	case *object.Int:
		return object.NewFloat(float64(x.V)), nil
	case object.Bool:
		if x {
			return object.NewFloat(1), nil
		}
		return object.NewFloat(0), nil
	case *object.Str:
		v, err := strconv.ParseFloat(strings.TrimSpace(x.V), 64)
		if err != nil {
			return nil, object.Raisef(object.ValueErrorClass, "could not convert string to float: '%s'", x.V)
		}
		return object.NewFloat(v), nil
	}
	return nil, object.Raisef(object.TypeErrorClass, "float() argument must be a string or a number, not '%s'", args[0].TypeName())
}

func builtinDict(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	d := object.NewDict()
	if len(args) > 1 {
		return nil, object.Raisef(object.TypeErrorClass, "dict() takes at most 1 positional argument (%d given)", len(args))
	}
	if len(args) == 1 {
		switch x := args[0].(type) {
		case *object.Dict:
			for idx, k := range x.Keys {
				if err := d.Set(k, x.Values[idx]); err != nil {
					return nil, err
				}
			}
		default:
			it, err := object.Iterate(x)
			if err != nil {
				return nil, err
			}
			for {
				pair, err := it.Next()
				if err == object.ErrIterationDone {
					break
				}
				if err != nil {
					return nil, err
				}
				vals, err := sequenceValues(pair)
				if err != nil || len(vals) != 2 {
					return nil, object.Raisef(object.ValueErrorClass, "dictionary update sequence element has length other than 2")
				}
				if err := d.Set(vals[0], vals[1]); err != nil {
					return nil, err
				}
			}
		}
	}
	for k, v := range kwargs {
		if err := d.Set(object.NewStr(k), v); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func builtinRange(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if err := noKwargs("range", kwargs); err != nil {
		return nil, err
	}
	if len(args) < 1 || len(args) > 3 {
		return nil, object.Raisef(object.TypeErrorClass, "range expected 1 to 3 arguments, got %d", len(args))
	}
	vals := make([]int64, len(args))
	for idx, a := range args {
		v, ok := toInt(a)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "'%s' object cannot be interpreted as an integer", a.TypeName())
		}
		vals[idx] = v
	}
	start, stop, step := int64(0), int64(0), int64(1)
	switch len(vals) {
	case 1:
		stop = vals[0]
	case 2:
		start, stop = vals[0], vals[1]
	case 3:
		start, stop, step = vals[0], vals[1], vals[2]
	}
	if step == 0 {
		return nil, object.Raisef(object.ValueErrorClass, "range() arg 3 must not be zero")
	}
	return object.NewRange(start, stop, step), nil
}

func iterableValues(v object.Object) ([]object.Object, error) {
	it, err := object.Iterate(v)
	if err != nil {
		return nil, err
	}
	var out []object.Object
	for {
		x, err := it.Next()
		if err == object.ErrIterationDone {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
}

func toInt(o object.Object) (int64, bool) {
	switch x := o.(type) {
	case *object.Int:
		return x.V, true
	case object.Bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
