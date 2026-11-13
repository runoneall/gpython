// Package builtins provides the Python builtin functions and
// exception classes exposed in every module's global scope.
package builtins

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"gpython/internal/object"
)

// Scope returns a fresh builtin namespace.
func Scope() map[string]object.Object {
	m := map[string]object.Object{
		"None":       object.None,
		"True":       object.True,
		"False":      object.False,
		"Ellipsis":   object.Ellipsis,
		"object":     object.ObjectClass,
		"type":       object.TypeClass,
		"int":        object.IntClass,
		"float":      object.FloatClass,
		"bool":       object.BoolClass,
		"str":        object.StrClass,
		"list":       object.ListClass,
		"tuple":      object.TupleClass,
		"dict":       object.DictClass,
		"set":        object.SetClass,
		"range":      object.RangeClass,
		"print":      newPrint(),
		"len":        newLen(),
		"isinstance": newIsInstance(),
		"issubclass": newIsSubclass(),
		"repr":       newRepr(),
		"abs":        newAbs(),
		"min":        newMinMax("min", false),
		"max":        newMinMax("max", true),
		"sum":        newSum(),
		"sorted":     newSorted(),
		"enumerate":  newEnumerate(),
		"zip":        newZip(),
		"next":       newNext(),
		"iter":       newIter(),
		"super":      newSuper(),
		"callable":   newCallable(),
		"hasattr":    newHasAttr(),
		"getattr":    newGetAttr(),
		"setattr":    newSetAttr(),
		"ord":        newOrd(),
		"chr":        newChr(),
		"round":      newRound(),
		"hash":       newHash(),
	}
	registerExceptions(m)
	return m
}

func registerExceptions(m map[string]object.Object) {
	classes := []*object.Class{
		object.BaseExceptionClass, object.ExceptionClass,
		object.ArithmeticError, object.AssertionError, object.AttributeError,
		object.ImportError, object.ModuleNotFoundError,
		object.LookupError, object.IndexError, object.KeyError,
		object.NameError, object.NotImplementedError, object.OSError,
		object.OverflowError, object.RuntimeError, object.StopIteration,
		object.SyntaxErrorClass, object.TypeErrorClass, object.ValueErrorClass,
		object.ZeroDivisionError, object.KeyboardInterrupt, object.SystemExit,
		object.RecursionError,
	}
	for _, c := range classes {
		m[c.Name] = c
	}
}

func kwargsEmpty(name string, kwargs map[string]object.Object) error {
	if len(kwargs) > 0 {
		for k := range kwargs {
			return object.Raisef(object.TypeErrorClass, "%s() got an unexpected keyword argument '%s'", name, k)
		}
	}
	return nil
}

func argCount(name string, args []object.Object, min, max int) error {
	if len(args) < min {
		return object.Raisef(object.TypeErrorClass, "%s() takes at least %d argument(s) (%d given)", name, min, len(args))
	}
	if max >= 0 && len(args) > max {
		return object.Raisef(object.TypeErrorClass, "%s() takes at most %d argument(s) (%d given)", name, max, len(args))
	}
	return nil
}

func newPrint() *object.Builtin {
	return object.NewBuiltin("print", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		sep := " "
		end := "\n"
		for k, v := range kwargs {
			switch k {
			case "sep":
				if v == object.None {
					continue
				}
				s, ok := v.(*object.Str)
				if !ok {
					return nil, object.Raisef(object.TypeErrorClass, "sep must be None or a string")
				}
				sep = s.V
			case "end":
				if v == object.None {
					continue
				}
				s, ok := v.(*object.Str)
				if !ok {
					return nil, object.Raisef(object.TypeErrorClass, "end must be None or a string")
				}
				end = s.V
			case "flush":
				// accepted and ignored
			default:
				return nil, object.Raisef(object.TypeErrorClass, "print() got an unexpected keyword argument '%s'", k)
			}
		}
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.Str()
		}
		fmt.Fprint(os.Stdout, strings.Join(parts, sep)+end)
		return object.None, nil
	})
}

func newLen() *object.Builtin {
	return object.NewBuiltin("len", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("len", args, 1, 1); err != nil {
			return nil, err
		}
		n, err := object.Len(args[0])
		if err != nil {
			return nil, err
		}
		return object.NewInt(int64(n)), nil
	})
}

func newIsInstance() *object.Builtin {
	return object.NewBuiltin("isinstance", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("isinstance", args, 2, 2); err != nil {
			return nil, err
		}
		ok, err := object.IsInstance(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return object.BoolFrom(ok), nil
	})
}

func newIsSubclass() *object.Builtin {
	return object.NewBuiltin("issubclass", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("issubclass", args, 2, 2); err != nil {
			return nil, err
		}
		ok, err := object.IsSubclass(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return object.BoolFrom(ok), nil
	})
}

func newRepr() *object.Builtin {
	return object.NewBuiltin("repr", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("repr", args, 1, 1); err != nil {
			return nil, err
		}
		return object.NewStr(args[0].Repr()), nil
	})
}

func newAbs() *object.Builtin {
	return object.NewBuiltin("abs", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("abs", args, 1, 1); err != nil {
			return nil, err
		}
		switch x := args[0].(type) {
		case *object.Int:
			if x.V < 0 {
				return object.NewInt(-x.V), nil
			}
			return object.NewInt(x.V), nil
		case *object.Float:
			return object.NewFloat(math.Abs(x.V)), nil
		case object.Bool:
			return object.NewInt(int64(boolValue(x))), nil
		}
		return nil, object.Raisef(object.TypeErrorClass, "bad operand type for abs(): '%s'", args[0].TypeName())
	})
}

func newMinMax(name string, wantMax bool) *object.Builtin {
	return object.NewBuiltin(name, func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) == 0 {
			return nil, object.Raisef(object.TypeErrorClass, "%s expected at least 1 argument, got 0", name)
		}
		var keyFn object.Object
		if v, ok := kwargs["key"]; ok {
			keyFn = v
		} else if v, ok := kwargs["default"]; ok {
			_ = v
		}
		for k := range kwargs {
			if k != "key" && k != "default" {
				return nil, object.Raisef(object.TypeErrorClass, "%s() got an unexpected keyword argument '%s'", name, k)
			}
		}
		var items []object.Object
		if len(args) == 1 {
			it, err := object.Iterate(args[0])
			if err != nil {
				return nil, err
			}
			for {
				v, err := it.Next()
				if err == object.ErrIterationDone {
					break
				}
				if err != nil {
					return nil, err
				}
				items = append(items, v)
			}
		} else {
			items = args
		}
		if len(items) == 0 {
			if def, ok := kwargs["default"]; ok {
				return def, nil
			}
			return nil, object.Raisef(object.ValueErrorClass, "%s() arg is an empty sequence", name)
		}
		keyOf := func(o object.Object) (object.Object, error) {
			if keyFn == nil || keyFn == object.None {
				return o, nil
			}
			return ctx.Call(keyFn, []object.Object{o}, nil)
		}
		best := items[0]
		bestKey, err := keyOf(best)
		if err != nil {
			return nil, err
		}
		for _, item := range items[1:] {
			k, err := keyOf(item)
			if err != nil {
				return nil, err
			}
			less, err := object.Less(k, bestKey)
			if err != nil {
				return nil, err
			}
			if (wantMax && !less && !object.Equal(k, bestKey)) || (!wantMax && less) {
				best = item
				bestKey = k
			}
		}
		return best, nil
	})
}

func newSum() *object.Builtin {
	return object.NewBuiltin("sum", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("sum", args, 1, 2); err != nil {
			return nil, err
		}
		var acc object.Object = object.NewInt(0)
		if len(args) == 2 {
			acc = args[1]
		}
		it, err := object.Iterate(args[0])
		if err != nil {
			return nil, err
		}
		for {
			v, err := it.Next()
			if err == object.ErrIterationDone {
				break
			}
			if err != nil {
				return nil, err
			}
			acc, err = object.BinaryOp("Add", acc, v)
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	})
}

func newSorted() *object.Builtin {
	return object.NewBuiltin("sorted", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("sorted", args, 1, 1); err != nil {
			return nil, err
		}
		var keyFn object.Object
		reverse := false
		for k, v := range kwargs {
			switch k {
			case "key":
				keyFn = v
			case "reverse":
				reverse = v.Truthy()
			default:
				return nil, object.Raisef(object.TypeErrorClass, "sorted() got an unexpected keyword argument '%s'", k)
			}
		}
		var items []object.Object
		it, err := object.Iterate(args[0])
		if err != nil {
			return nil, err
		}
		for {
			v, err := it.Next()
			if err == object.ErrIterationDone {
				break
			}
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		}
		keyOf := func(o object.Object) (object.Object, error) {
			if keyFn == nil || keyFn == object.None {
				return o, nil
			}
			return ctx.Call(keyFn, []object.Object{o}, nil)
		}
		keys := make([]object.Object, len(items))
		for i, item := range items {
			k, err := keyOf(item)
			if err != nil {
				return nil, err
			}
			keys[i] = k
		}
		var sortErr error
		idx := make([]int, len(items))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool {
			if sortErr != nil {
				return false
			}
			var less bool
			var err error
			if reverse {
				less, err = object.Less(keys[idx[b]], keys[idx[a]])
			} else {
				less, err = object.Less(keys[idx[a]], keys[idx[b]])
			}
			if err != nil {
				sortErr = err
				return false
			}
			return less
		})
		if sortErr != nil {
			return nil, sortErr
		}
		out := make([]object.Object, len(items))
		for i, j := range idx {
			out[i] = items[j]
		}
		return object.NewList(out...), nil
	})
}

type enumerateIter struct {
	it  object.Iterator
	idx int64
}

func (e *enumerateIter) Next() (object.Object, error) {
	v, err := e.it.Next()
	if err != nil {
		return nil, err
	}
	pair := object.NewTuple(object.NewInt(e.idx), v)
	e.idx++
	return pair, nil
}

func newEnumerate() *object.Builtin {
	return object.NewBuiltin("enumerate", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("enumerate", args, 1, 1); err != nil {
			return nil, err
		}
		start := int64(0)
		if v, ok := kwargs["start"]; ok {
			i, ok := intValue(v)
			if !ok {
				return nil, object.Raisef(object.TypeErrorClass, "enumerate() start must be integer")
			}
			start = i
		}
		it, err := object.Iterate(args[0])
		if err != nil {
			return nil, err
		}
		return object.NewIterObject(&enumerateIter{it: it, idx: start}), nil
	})
}

type zipIter struct {
	its []object.Iterator
}

func (z *zipIter) Next() (object.Object, error) {
	out := make([]object.Object, len(z.its))
	for i, it := range z.its {
		v, err := it.Next()
		if err == object.ErrIterationDone {
			return nil, object.ErrIterationDone
		}
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return object.NewTuple(out...), nil
}

func newZip() *object.Builtin {
	return object.NewBuiltin("zip", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		iters := make([]object.Iterator, len(args))
		for i, a := range args {
			it, err := object.Iterate(a)
			if err != nil {
				return nil, err
			}
			iters[i] = it
		}
		return object.NewIterObject(&zipIter{its: iters}), nil
	})
}

func newNext() *object.Builtin {
	return object.NewBuiltin("next", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("next", args, 1, 2); err != nil {
			return nil, err
		}
		v, err := object.Next(args[0])
		if err == object.ErrIterationDone {
			if len(args) == 2 {
				return args[1], nil
			}
			return nil, object.Raise(object.StopIteration)
		}
		return v, err
	})
}

func newIter() *object.Builtin {
	return object.NewBuiltin("iter", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("iter", args, 1, 1); err != nil {
			return nil, err
		}
		if _, ok := args[0].(*object.Generator); ok {
			return args[0], nil
		}
		it, err := object.Iterate(args[0])
		if err != nil {
			return nil, err
		}
		return object.NewIterObject(it), nil
	})
}

func newSuper() *object.Builtin {
	return object.NewBuiltin("super", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 0 {
			return nil, object.Raisef(object.NotImplementedError, "super() with arguments is not implemented")
		}
		return ctx.Super()
	})
}

func newCallable() *object.Builtin {
	return object.NewBuiltin("callable", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("callable", args, 1, 1); err != nil {
			return nil, err
		}
		switch args[0].(type) {
		case *object.Function, *object.BoundMethod, *object.Builtin, *object.Class, object.Callable:
			return object.True, nil
		}
		return object.False, nil
	})
}

func newHasAttr() *object.Builtin {
	return object.NewBuiltin("hasattr", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("hasattr", args, 2, 2); err != nil {
			return nil, err
		}
		name, ok := args[1].(*object.Str)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "attribute name must be string")
		}
		if _, err := object.GetAttr(args[0], name.V); err != nil {
			return object.False, nil
		}
		return object.True, nil
	})
}

func newGetAttr() *object.Builtin {
	return object.NewBuiltin("getattr", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("getattr", args, 2, 3); err != nil {
			return nil, err
		}
		name, ok := args[1].(*object.Str)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "attribute name must be string")
		}
		v, err := object.GetAttr(args[0], name.V)
		if err != nil && len(args) == 3 {
			return args[2], nil
		}
		return v, err
	})
}

func newSetAttr() *object.Builtin {
	return object.NewBuiltin("setattr", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("setattr", args, 3, 3); err != nil {
			return nil, err
		}
		name, ok := args[1].(*object.Str)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "attribute name must be string")
		}
		if err := object.SetAttr(args[0], name.V, args[2]); err != nil {
			return nil, err
		}
		return object.None, nil
	})
}

func newOrd() *object.Builtin {
	return object.NewBuiltin("ord", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("ord", args, 1, 1); err != nil {
			return nil, err
		}
		s, ok := args[0].(*object.Str)
		if !ok || utf8.RuneCountInString(s.V) != 1 {
			return nil, object.Raisef(object.TypeErrorClass, "ord() expected a character")
		}
		r, _ := utf8.DecodeRuneInString(s.V)
		return object.NewInt(int64(r)), nil
	})
}

func newChr() *object.Builtin {
	return object.NewBuiltin("chr", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("chr", args, 1, 1); err != nil {
			return nil, err
		}
		i, ok := intValue(args[0])
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "chr() argument must be integer")
		}
		return object.NewStr(string(rune(i))), nil
	})
}

func newRound() *object.Builtin {
	return object.NewBuiltin("round", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("round", args, 1, 2); err != nil {
			return nil, err
		}
		nd := int64(0)
		if len(args) == 2 {
			v, ok := intValue(args[1])
			if !ok {
				return nil, object.Raisef(object.TypeErrorClass, "round() ndigits must be integer")
			}
			nd = v
		}
		switch x := args[0].(type) {
		case *object.Int:
			if nd >= 0 {
				return object.NewInt(x.V), nil
			}
			p := math.Pow(10, float64(-nd))
			r := math.Round(float64(x.V)/p) * p
			return object.NewInt(int64(r)), nil
		case *object.Float:
			p := math.Pow(10, float64(nd))
			r := math.Round(x.V*p) / p
			if nd <= 0 {
				return object.NewFloat(r), nil
			}
			return object.NewFloat(r), nil
		}
		return nil, object.Raisef(object.TypeErrorClass, "type %s doesn't define __round__ method", args[0].TypeName())
	})
}

func newHash() *object.Builtin {
	return object.NewBuiltin("hash", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if err := argCount("hash", args, 1, 1); err != nil {
			return nil, err
		}
		if !object.Hashable(args[0]) {
			return nil, object.Raisef(object.TypeErrorClass, "unhashable type: '%s'", args[0].TypeName())
		}
		switch x := args[0].(type) {
		case *object.Int:
			return object.NewInt(x.V), nil
		case object.Bool:
			return object.NewInt(int64(boolValue(x))), nil
		case *object.Str:
			var h int64
			for _, r := range x.V {
				h = h*31 + int64(r)
			}
			return object.NewInt(h), nil
		}
		return object.NewInt(0), nil
	})
}

func boolValue(b object.Bool) int {
	if b {
		return 1
	}
	return 0
}

func intValue(o object.Object) (int64, bool) {
	switch x := o.(type) {
	case *object.Int:
		return x.V, true
	case object.Bool:
		return int64(boolValue(x)), true
	case *object.Float:
		return int64(x.V), true
	case *object.Str:
		v, err := strconv.ParseInt(strings.TrimSpace(x.V), 10, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	return 0, false
}
