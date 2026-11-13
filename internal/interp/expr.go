package interp

import (
	"strings"

	"gpython/internal/object"

	"github.com/tamnd/gopapy/v2/parser2"
)

func (i *Interpreter) evalExpr(e parser2.Expr, env *Env) (object.Object, error) {
	i.Pos = nodePos(e)
	switch n := e.(type) {
	case *parser2.Constant:
		return constantValue(n)
	case *parser2.Name:
		return i.lookupName(env, n.Id)
	case *parser2.UnaryOp:
		if n.Op == "Not" {
			v, err := i.evalExpr(n.Operand, env)
			if err != nil {
				return nil, err
			}
			return object.BoolFrom(!v.Truthy()), nil
		}
		v, err := i.evalExpr(n.Operand, env)
		if err != nil {
			return nil, err
		}
		return object.UnaryOp(n.Op, v)
	case *parser2.BinOp:
		l, err := i.evalExpr(n.Left, env)
		if err != nil {
			return nil, err
		}
		r, err := i.evalExpr(n.Right, env)
		if err != nil {
			return nil, err
		}
		return object.BinaryOp(n.Op, l, r)
	case *parser2.BoolOp:
		return i.evalBoolOp(n, env)
	case *parser2.Compare:
		return i.evalCompare(n, env)
	case *parser2.Attribute:
		v, err := i.evalExpr(n.Value, env)
		if err != nil {
			return nil, err
		}
		return object.GetAttr(v, n.Attr)
	case *parser2.Subscript:
		v, err := i.evalExpr(n.Value, env)
		if err != nil {
			return nil, err
		}
		key, err := i.evalSubscriptKey(n.Slice, env)
		if err != nil {
			return nil, err
		}
		return object.GetItem(v, key)
	case *parser2.Call:
		return i.evalCall(n, env)
	case *parser2.List:
		elts, err := i.evalElements(n.Elts, env)
		if err != nil {
			return nil, err
		}
		return object.NewList(elts...), nil
	case *parser2.Tuple:
		elts, err := i.evalElements(n.Elts, env)
		if err != nil {
			return nil, err
		}
		return object.NewTuple(elts...), nil
	case *parser2.Set:
		elts, err := i.evalElements(n.Elts, env)
		if err != nil {
			return nil, err
		}
		set := object.NewSet()
		for _, elt := range elts {
			if err := set.Add(elt); err != nil {
				return nil, err
			}
		}
		return set, nil
	case *parser2.Dict:
		return i.evalDictLiteral(n, env)
	case *parser2.Lambda:
		defaults, kwDefaults, err := i.evalDefaults(n.Args, env)
		if err != nil {
			return nil, err
		}
		return &object.Function{
			Name:       "<lambda>",
			Args:       n.Args,
			Defaults:   defaults,
			KwDefaults: kwDefaults,
			ExprBody:   n.Body,
			Closure:    env,
			File:       i.currentFile(),
			IsLambda:   true,
		}, nil
	case *parser2.IfExp:
		test, err := i.evalExpr(n.Test, env)
		if err != nil {
			return nil, err
		}
		if test.Truthy() {
			return i.evalExpr(n.Body, env)
		}
		return i.evalExpr(n.OrElse, env)
	case *parser2.NamedExpr:
		v, err := i.evalExpr(n.Value, env)
		if err != nil {
			return nil, err
		}
		if err := i.assignTarget(n.Target, v, env); err != nil {
			return nil, err
		}
		return v, nil
	case *parser2.Starred:
		return i.evalExpr(n.Value, env)
	case *parser2.ListComp:
		return i.evalListComp(n, env)
	case *parser2.SetComp:
		return i.evalSetComp(n, env)
	case *parser2.DictComp:
		return i.evalDictComp(n, env)
	case *parser2.GeneratorExp:
		return i.evalGeneratorExp(n, env)
	case *parser2.Yield:
		return i.evalYield(n, env)
	case *parser2.YieldFrom:
		return i.evalYieldFrom(n, env)
	case *parser2.JoinedStr:
		return i.evalJoinedStr(n, env)
	case *parser2.FormattedValue:
		return i.evalFormattedValue(n, env)
	case *parser2.Await:
		return nil, &FatalError{File: i.currentFile(), Line: n.P.Line, Col: n.P.Col, Msg: "notimplementederror: async/await is not supported"}
	case *parser2.TemplateStr, *parser2.Interpolation:
		return nil, object.Raisef(object.NotImplementedError, "template strings are not supported")
	}
	return nil, object.Raisef(object.NotImplementedError, "expression is not supported")
}

func (i *Interpreter) currentFile() string {
	if i.Curr != nil {
		return i.Curr.File
	}
	return ""
}

func constantValue(c *parser2.Constant) (object.Object, error) {
	switch c.Kind {
	case "int":
		switch v := c.Value.(type) {
		case int64:
			return object.NewInt(v), nil
		case int:
			return object.NewInt(int64(v)), nil
		}
		return nil, object.Raisef(object.RuntimeError, "invalid integer literal")
	case "float":
		if v, ok := c.Value.(float64); ok {
			return object.NewFloat(v), nil
		}
		return nil, object.Raisef(object.RuntimeError, "invalid float literal")
	case "str":
		if v, ok := c.Value.(string); ok {
			return object.NewStr(v), nil
		}
		return nil, object.Raisef(object.RuntimeError, "invalid string literal")
	case "bytes":
		if v, ok := c.Value.(string); ok {
			return object.NewStr(v), nil
		}
		return nil, object.Raisef(object.RuntimeError, "invalid bytes literal")
	case "True":
		return object.True, nil
	case "False":
		return object.False, nil
	case "None":
		return object.None, nil
	case "Ellipsis":
		return object.Ellipsis, nil
	case "complex":
		return nil, object.Raisef(object.NotImplementedError, "complex numbers are not supported")
	}
	return nil, object.Raisef(object.NotImplementedError, "literal kind '%s' is not supported", c.Kind)
}

func (i *Interpreter) evalBoolOp(n *parser2.BoolOp, env *Env) (object.Object, error) {
	var last object.Object = object.None
	for _, ve := range n.Values {
		v, err := i.evalExpr(ve, env)
		if err != nil {
			return nil, err
		}
		last = v
		if n.Op == "And" {
			if !v.Truthy() {
				return v, nil
			}
		} else if v.Truthy() {
			return v, nil
		}
	}
	return last, nil
}

func (i *Interpreter) evalCompare(n *parser2.Compare, env *Env) (object.Object, error) {
	left, err := i.evalExpr(n.Left, env)
	if err != nil {
		return nil, err
	}
	for idx, op := range n.Ops {
		right, err := i.evalExpr(n.Comparators[idx], env)
		if err != nil {
			return nil, err
		}
		res, err := object.Compare(op, left, right)
		if err != nil {
			return nil, err
		}
		if !res.Truthy() {
			return object.False, nil
		}
		left = right
	}
	return object.True, nil
}

func (i *Interpreter) evalSubscriptKey(e parser2.Expr, env *Env) (object.Object, error) {
	if sl, ok := e.(*parser2.Slice); ok {
		key := &object.Slice{}
		var err error
		if sl.Lower != nil {
			key.Lower, err = i.evalExpr(sl.Lower, env)
			if err != nil {
				return nil, err
			}
		}
		if sl.Upper != nil {
			key.Upper, err = i.evalExpr(sl.Upper, env)
			if err != nil {
				return nil, err
			}
		}
		if sl.Step != nil {
			key.Step, err = i.evalExpr(sl.Step, env)
			if err != nil {
				return nil, err
			}
		}
		return key, nil
	}
	return i.evalExpr(e, env)
}

func (i *Interpreter) evalElements(exprs []parser2.Expr, env *Env) ([]object.Object, error) {
	out := make([]object.Object, 0, len(exprs))
	for _, e := range exprs {
		if s, ok := e.(*parser2.Starred); ok {
			v, err := i.evalExpr(s.Value, env)
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
				out = append(out, x)
			}
			continue
		}
		v, err := i.evalExpr(e, env)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (i *Interpreter) evalDictLiteral(n *parser2.Dict, env *Env) (object.Object, error) {
	d := object.NewDict()
	for idx, ke := range n.Keys {
		ve := n.Values[idx]
		if ke == nil {
			other, err := i.evalExpr(ve, env)
			if err != nil {
				return nil, err
			}
			m, ok := other.(*object.Dict)
			if !ok {
				return nil, object.Raisef(object.TypeErrorClass, "'%s' object is not a mapping", other.TypeName())
			}
			for j, k := range m.Keys {
				if err := d.Set(k, m.Values[j]); err != nil {
					return nil, err
				}
			}
			continue
		}
		k, err := i.evalExpr(ke, env)
		if err != nil {
			return nil, err
		}
		v, err := i.evalExpr(ve, env)
		if err != nil {
			return nil, err
		}
		if err := d.Set(k, v); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (i *Interpreter) evalListComp(n *parser2.ListComp, env *Env) (object.Object, error) {
	comp := env.Child(EnvFunction, "<listcomp>")
	out := object.NewList()
	err := i.runComp(n.Gens, 0, comp, env, func() error {
		v, err := i.evalExpr(n.Elt, comp)
		if err != nil {
			return err
		}
		out.Elts = append(out.Elts, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (i *Interpreter) evalSetComp(n *parser2.SetComp, env *Env) (object.Object, error) {
	comp := env.Child(EnvFunction, "<setcomp>")
	out := object.NewSet()
	err := i.runComp(n.Gens, 0, comp, env, func() error {
		v, err := i.evalExpr(n.Elt, comp)
		if err != nil {
			return err
		}
		return out.Add(v)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (i *Interpreter) evalDictComp(n *parser2.DictComp, env *Env) (object.Object, error) {
	comp := env.Child(EnvFunction, "<dictcomp>")
	out := object.NewDict()
	err := i.runComp(n.Gens, 0, comp, env, func() error {
		k, err := i.evalExpr(n.Key, comp)
		if err != nil {
			return err
		}
		v, err := i.evalExpr(n.Value, comp)
		if err != nil {
			return err
		}
		return out.Set(k, v)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (i *Interpreter) evalGeneratorExp(n *parser2.GeneratorExp, env *Env) (object.Object, error) {
	outer := env
	sub := i.clone()
	return object.NewGenerator("<genexpr>", func(g *object.Generator) error {
		sub.gen = g
		comp := outer.Child(EnvFunction, "<genexpr>")
		return sub.runComp(n.Gens, 0, comp, outer, func() error {
			v, err := sub.evalExpr(n.Elt, comp)
			if err != nil {
				return err
			}
			g.Yield(v)
			return nil
		})
	}), nil
}

func (i *Interpreter) runComp(gens []*parser2.Comprehension, idx int, comp, outer *Env, body func() error) error {
	if idx == len(gens) {
		return body()
	}
	g := gens[idx]
	iterEnv := comp
	if idx == 0 {
		iterEnv = outer
	}
	iv, err := i.evalExpr(g.Iter, iterEnv)
	if err != nil {
		return err
	}
	it, err := object.Iterate(iv)
	if err != nil {
		return err
	}
	for {
		v, err := it.Next()
		if err == object.ErrIterationDone {
			break
		}
		if err != nil {
			return err
		}
		if err := i.assignTarget(g.Target, v, comp); err != nil {
			return err
		}
		ok := true
		for _, cond := range g.Ifs {
			cv, err := i.evalExpr(cond, comp)
			if err != nil {
				return err
			}
			if !cv.Truthy() {
				ok = false
				break
			}
		}
		if ok {
			if err := i.runComp(gens, idx+1, comp, outer, body); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *Interpreter) evalYield(n *parser2.Yield, env *Env) (object.Object, error) {
	if i.gen == nil {
		return nil, object.Raisef(object.SyntaxErrorClass, "'yield' outside function")
	}
	var v object.Object = object.None
	if n.Value != nil {
		var err error
		v, err = i.evalExpr(n.Value, env)
		if err != nil {
			return nil, err
		}
	}
	i.gen.Yield(v)
	return object.None, nil
}

func (i *Interpreter) evalYieldFrom(n *parser2.YieldFrom, env *Env) (object.Object, error) {
	if i.gen == nil {
		return nil, object.Raisef(object.SyntaxErrorClass, "'yield from' outside function")
	}
	v, err := i.evalExpr(n.Value, env)
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
			return object.None, nil
		}
		if err != nil {
			return nil, err
		}
		i.gen.Yield(x)
	}
}

func (i *Interpreter) evalJoinedStr(n *parser2.JoinedStr, env *Env) (object.Object, error) {
	var b strings.Builder
	for _, ve := range n.Values {
		v, err := i.evalExpr(ve, env)
		if err != nil {
			return nil, err
		}
		b.WriteString(v.Str())
	}
	return object.NewStr(b.String()), nil
}

func (i *Interpreter) evalFormattedValue(n *parser2.FormattedValue, env *Env) (object.Object, error) {
	v, err := i.evalExpr(n.Value, env)
	if err != nil {
		return nil, err
	}
	spec := ""
	if n.FormatSpec != nil {
		sv, err := i.evalExpr(n.FormatSpec, env)
		if err != nil {
			return nil, err
		}
		spec = sv.Str()
	}
	if n.Conversion != -1 {
		var converted string
		switch n.Conversion {
		case 114: // !r
			converted = v.Repr()
		case 115: // !s
			converted = v.Str()
		case 97: // !a
			converted = asciiRepr(v.Repr())
		default:
			converted = v.Str()
		}
		if spec == "" {
			return object.NewStr(converted), nil
		}
		v = object.NewStr(converted)
	}
	if spec != "" {
		s, err := object.FormatValue(v, spec)
		if err != nil {
			return nil, err
		}
		return object.NewStr(s), nil
	}
	return object.NewStr(v.Str()), nil
}

// asciiRepr escapes non-ASCII runes like Python's !a conversion.
func asciiRepr(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 {
			b.WriteRune(r)
		} else if r <= 0xffff {
			b.WriteString("\\u")
			b.WriteString(padHex(int64(r), 4))
		} else {
			b.WriteString("\\U")
			b.WriteString(padHex(int64(r), 8))
		}
	}
	return b.String()
}

func padHex(v int64, width int) string {
	const hex = "0123456789abcdef"
	buf := make([]byte, width)
	for j := width - 1; j >= 0; j-- {
		buf[j] = hex[v&0xf]
		v >>= 4
	}
	return string(buf)
}
