package object

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Slice is a subscript slice expression: a[lo:hi:step].
type Slice struct {
	Lower Object
	Upper Object
	Step  Object
}

func (*Slice) TypeName() string { return "slice" }
func (s *Slice) Repr() string {
	part := func(o Object) string {
		if o == nil {
			return ""
		}
		return o.Repr()
	}
	if s.Step != nil {
		return "slice(" + part(s.Lower) + ", " + part(s.Upper) + ", " + part(s.Step) + ")"
	}
	return "slice(" + part(s.Lower) + ", " + part(s.Upper) + ", None)"
}
func (s *Slice) Str() string { return s.Repr() }
func (*Slice) Truthy() bool  { return true }

func asInt(o Object) (int64, bool) {
	switch x := o.(type) {
	case *Int:
		return x.V, true
	case Bool:
		return int64(boolInt(x)), true
	}
	return 0, false
}

func sliceBounds(n int, s *Slice) (start, stop, step int, err error) {
	step = 1
	if s.Step != nil {
		v, ok := asInt(s.Step)
		if !ok {
			return 0, 0, 0, Raisef(TypeErrorClass, "slice indices must be integers or None")
		}
		step = int(v)
	}
	if step == 0 {
		return 0, 0, 0, Raisef(ValueErrorClass, "slice step cannot be zero")
	}
	lower := func(def int) (int, error) {
		if s.Lower == nil {
			return def, nil
		}
		v, ok := asInt(s.Lower)
		if !ok {
			return 0, Raisef(TypeErrorClass, "slice indices must be integers or None")
		}
		return int(v), nil
	}
	upper := func(def int) (int, error) {
		if s.Upper == nil {
			return def, nil
		}
		v, ok := asInt(s.Upper)
		if !ok {
			return 0, Raisef(TypeErrorClass, "slice indices must be integers or None")
		}
		return int(v), nil
	}
	if step > 0 {
		start, err = lower(0)
		if err != nil {
			return
		}
		stop, err = upper(n)
		if err != nil {
			return
		}
		start = clamp(start, n, false)
		stop = clamp(stop, n, false)
	} else {
		start, err = lower(n - 1)
		if err != nil {
			return
		}
		stop, err = upper(-1)
		if err != nil {
			return
		}
		start = clamp(start, n, true)
		stop = clamp(stop, n, true)
	}
	return
}

func clamp(i, n int, negStep bool) int {
	if i < 0 {
		i += n
	}
	if negStep {
		if i < -1 {
			return -1
		}
		if i >= n {
			return n - 1
		}
		return i
	}
	if i < 0 {
		return 0
	}
	if i > n {
		return n
	}
	return i
}

func sliceIndices(n int, s *Slice, adjust func(int) int) ([]int, error) {
	start, stop, step, err := sliceBounds(n, s)
	if err != nil {
		return nil, err
	}
	var out []int
	if step > 0 {
		for i := start; i < stop; i += step {
			out = append(out, adjust(i))
		}
	} else {
		for i := start; i > stop; i += step {
			out = append(out, adjust(i))
		}
	}
	return out, nil
}

// Len implements len().
func Len(o Object) (int, error) {
	switch x := o.(type) {
	case *List:
		return len(x.Elts), nil
	case *Tuple:
		return len(x.Elts), nil
	case *Dict:
		return len(x.Keys), nil
	case *Set:
		return len(x.Elts), nil
	case *Str:
		return utf8.RuneCountInString(x.V), nil
	case *Range:
		return int(x.Len()), nil
	}
	return 0, Raisef(TypeErrorClass, "object of type '%s' has no len()", o.TypeName())
}

func normalizeIndex(i int64, n int) (int, bool) {
	idx := int(i)
	if idx < 0 {
		idx += n
	}
	if idx < 0 || idx >= n {
		return 0, false
	}
	return idx, true
}

// GetItem implements o[key].
func GetItem(o Object, key Object) (Object, error) {
	switch x := o.(type) {
	case *List:
		switch k := key.(type) {
		case *Slice:
			idx, err := sliceIndices(len(x.Elts), k, func(i int) int { return i })
			if err != nil {
				return nil, err
			}
			out := make([]Object, len(idx))
			for i, j := range idx {
				out[i] = x.Elts[j]
			}
			return NewList(out...), nil
		}
		i, ok := asInt(key)
		if !ok {
			return nil, Raisef(TypeErrorClass, "list indices must be integers or slices, not '%s'", key.TypeName())
		}
		idx, ok := normalizeIndex(i, len(x.Elts))
		if !ok {
			return nil, Raisef(IndexError, "list index out of range")
		}
		return x.Elts[idx], nil
	case *Tuple:
		switch k := key.(type) {
		case *Slice:
			idx, err := sliceIndices(len(x.Elts), k, func(i int) int { return i })
			if err != nil {
				return nil, err
			}
			out := make([]Object, len(idx))
			for i, j := range idx {
				out[i] = x.Elts[j]
			}
			return NewTuple(out...), nil
		}
		i, ok := asInt(key)
		if !ok {
			return nil, Raisef(TypeErrorClass, "tuple indices must be integers or slices, not '%s'", key.TypeName())
		}
		idx, ok := normalizeIndex(i, len(x.Elts))
		if !ok {
			return nil, Raisef(IndexError, "tuple index out of range")
		}
		return x.Elts[idx], nil
	case *Str:
		runes := []rune(x.V)
		switch k := key.(type) {
		case *Slice:
			idx, err := sliceIndices(len(runes), k, func(i int) int { return i })
			if err != nil {
				return nil, err
			}
			var b strings.Builder
			for _, j := range idx {
				b.WriteRune(runes[j])
			}
			return NewStr(b.String()), nil
		}
		i, ok := asInt(key)
		if !ok {
			return nil, Raisef(TypeErrorClass, "string indices must be integers, not '%s'", key.TypeName())
		}
		idx, ok := normalizeIndex(i, len(runes))
		if !ok {
			return nil, Raisef(IndexError, "string index out of range")
		}
		return NewStr(string(runes[idx])), nil
	case *Dict:
		v, ok := x.Get(key)
		if !ok {
			return nil, Raise(KeyError, key)
		}
		return v, nil
	case *Range:
		i, ok := asInt(key)
		if !ok {
			return nil, Raisef(TypeErrorClass, "range indices must be integers")
		}
		idx := i
		if idx < 0 {
			idx += x.Len()
		}
		if idx < 0 || idx >= x.Len() {
			return nil, Raisef(IndexError, "range object index out of range")
		}
		return NewInt(x.Start + idx*x.Step), nil
	}
	return nil, Raisef(TypeErrorClass, "'%s' object is not subscriptable", o.TypeName())
}

// SetItem implements o[key] = value.
func SetItem(o Object, key, value Object) error {
	switch x := o.(type) {
	case *List:
		if k, ok := key.(*Slice); ok {
			idx, err := sliceIndices(len(x.Elts), k, func(i int) int { return i })
			if err != nil {
				return err
			}
			vals, ok := value.(*List)
			if !ok {
				if t, ok := value.(*Tuple); ok {
					vals = &List{Elts: t.Elts}
				} else {
					return Raisef(TypeErrorClass, "can only assign an iterable")
				}
			}
			if k.Step == nil || (isOne(k.Step)) {
				start := 0
				stop := 0
				if len(idx) > 0 {
					start = idx[0]
					stop = idx[len(idx)-1] + 1
				} else if k.Lower != nil {
					v, _ := asInt(k.Lower)
					start = clamp(int(v), len(x.Elts), false)
					stop = start
				}
				out := make([]Object, 0, len(x.Elts)-len(idx)+len(vals.Elts))
				out = append(out, x.Elts[:start]...)
				out = append(out, vals.Elts...)
				out = append(out, x.Elts[stop:]...)
				x.Elts = out
				return nil
			}
			if len(vals.Elts) != len(idx) {
				return Raisef(ValueErrorClass, "attempt to assign sequence of size %d to extended slice of size %d", len(vals.Elts), len(idx))
			}
			for i, j := range idx {
				x.Elts[j] = vals.Elts[i]
			}
			return nil
		}
		i, ok := asInt(key)
		if !ok {
			return Raisef(TypeErrorClass, "list indices must be integers or slices, not '%s'", key.TypeName())
		}
		idx, ok := normalizeIndex(i, len(x.Elts))
		if !ok {
			return Raisef(IndexError, "list assignment index out of range")
		}
		x.Elts[idx] = value
		return nil
	case *Dict:
		return x.Set(key, value)
	}
	return Raisef(TypeErrorClass, "'%s' object does not support item assignment", o.TypeName())
}

func isOne(o Object) bool {
	v, ok := asInt(o)
	return ok && v == 1
}

// DelItem implements del o[key].
func DelItem(o Object, key Object) error {
	switch x := o.(type) {
	case *List:
		if k, ok := key.(*Slice); ok {
			idx, err := sliceIndices(len(x.Elts), k, func(i int) int { return i })
			if err != nil {
				return err
			}
			drop := make(map[int]bool, len(idx))
			for _, i := range idx {
				drop[i] = true
			}
			out := make([]Object, 0, len(x.Elts)-len(idx))
			for i, v := range x.Elts {
				if !drop[i] {
					out = append(out, v)
				}
			}
			x.Elts = out
			return nil
		}
		i, ok := asInt(key)
		if !ok {
			return Raisef(TypeErrorClass, "list indices must be integers or slices, not '%s'", key.TypeName())
		}
		idx, ok := normalizeIndex(i, len(x.Elts))
		if !ok {
			return Raisef(IndexError, "list assignment index out of range")
		}
		x.Elts = append(x.Elts[:idx], x.Elts[idx+1:]...)
		return nil
	case *Dict:
		_, err := x.Delete(key)
		return err
	}
	return Raisef(TypeErrorClass, "'%s' object does not support item deletion", o.TypeName())
}

// Iterate returns an iterator over o.
func Iterate(o Object) (Iterator, error) {
	switch x := o.(type) {
	case *List:
		return &listIter{list: x}, nil
	case *Tuple:
		return &tupleIter{tuple: x}, nil
	case *Str:
		return &strIter{runes: []rune(x.V)}, nil
	case *Dict:
		return &dictIter{dict: x}, nil
	case *Set:
		return &setIter{set: x}, nil
	case *Range:
		return &rangeIter{r: x, cur: x.Start}, nil
	case *Generator:
		return x, nil
	case *IterObject:
		return x.It, nil
	}
	return nil, Raisef(TypeErrorClass, "'%s' object is not iterable", o.TypeName())
}

// Next implements the next() builtin protocol.
func Next(o Object) (Object, error) {
	switch x := o.(type) {
	case *Generator:
		return x.Next()
	case *IterObject:
		return x.Next()
	}
	if it, ok := o.(Iterator); ok {
		return it.Next()
	}
	return nil, Raisef(TypeErrorClass, "'%s' object is not an iterator", o.TypeName())
}

type listIter struct {
	list *List
	i    int
}

func (it *listIter) Next() (Object, error) {
	if it.i >= len(it.list.Elts) {
		return nil, ErrIterationDone
	}
	v := it.list.Elts[it.i]
	it.i++
	return v, nil
}

type tupleIter struct {
	tuple *Tuple
	i     int
}

func (it *tupleIter) Next() (Object, error) {
	if it.i >= len(it.tuple.Elts) {
		return nil, ErrIterationDone
	}
	v := it.tuple.Elts[it.i]
	it.i++
	return v, nil
}

type strIter struct {
	runes []rune
	i     int
}

func (it *strIter) Next() (Object, error) {
	if it.i >= len(it.runes) {
		return nil, ErrIterationDone
	}
	v := NewStr(string(it.runes[it.i]))
	it.i++
	return v, nil
}

type dictIter struct {
	dict *Dict
	i    int
}

func (it *dictIter) Next() (Object, error) {
	if it.i >= len(it.dict.Keys) {
		return nil, ErrIterationDone
	}
	v := it.dict.Keys[it.i]
	it.i++
	return v, nil
}

type setIter struct {
	set *Set
	i   int
}

func (it *setIter) Next() (Object, error) {
	if it.i >= len(it.set.Elts) {
		return nil, ErrIterationDone
	}
	v := it.set.Elts[it.i]
	it.i++
	return v, nil
}

type rangeIter struct {
	r   *Range
	cur int64
}

func (it *rangeIter) Next() (Object, error) {
	if it.r.Step > 0 && it.cur >= it.r.Stop {
		return nil, ErrIterationDone
	}
	if it.r.Step < 0 && it.cur <= it.r.Stop {
		return nil, ErrIterationDone
	}
	v := it.cur
	it.cur += it.r.Step
	return NewInt(v), nil
}

func listAttr(l *List, name string) (Object, error) {
	switch name {
	case "append":
		return NewBuiltin("append", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "append() takes exactly one argument (%d given)", len(args))
			}
			l.Elts = append(l.Elts, args[0])
			return None, nil
		}), nil
	case "extend":
		return NewBuiltin("extend", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "extend() takes exactly one argument (%d given)", len(args))
			}
			it, err := Iterate(args[0])
			if err != nil {
				return nil, err
			}
			for {
				v, err := it.Next()
				if err == ErrIterationDone {
					break
				}
				if err != nil {
					return nil, err
				}
				l.Elts = append(l.Elts, v)
			}
			return None, nil
		}), nil
	case "insert":
		return NewBuiltin("insert", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 2 {
				return nil, Raisef(TypeErrorClass, "insert() takes exactly 2 arguments (%d given)", len(args))
			}
			i, ok := asInt(args[0])
			if !ok {
				return nil, Raisef(TypeErrorClass, "insert() argument 1 must be integer")
			}
			idx := int(i)
			if idx < 0 {
				idx += len(l.Elts)
			}
			if idx < 0 {
				idx = 0
			}
			if idx > len(l.Elts) {
				idx = len(l.Elts)
			}
			l.Elts = append(l.Elts, nil)
			copy(l.Elts[idx+1:], l.Elts[idx:])
			l.Elts[idx] = args[1]
			return None, nil
		}), nil
	case "pop":
		return NewBuiltin("pop", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(l.Elts) == 0 {
				return nil, Raisef(IndexError, "pop from empty list")
			}
			idx := len(l.Elts) - 1
			if len(args) > 0 {
				i, ok := asInt(args[0])
				if !ok {
					return nil, Raisef(TypeErrorClass, "pop() argument must be integer")
				}
				j, ok := normalizeIndex(i, len(l.Elts))
				if !ok {
					return nil, Raisef(IndexError, "pop index out of range")
				}
				idx = j
			}
			v := l.Elts[idx]
			l.Elts = append(l.Elts[:idx], l.Elts[idx+1:]...)
			return v, nil
		}), nil
	case "remove":
		return NewBuiltin("remove", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "remove() takes exactly one argument (%d given)", len(args))
			}
			for i, v := range l.Elts {
				if Equal(v, args[0]) {
					l.Elts = append(l.Elts[:i], l.Elts[i+1:]...)
					return None, nil
				}
			}
			return nil, Raisef(ValueErrorClass, "%s is not in list", args[0].Repr())
		}), nil
	case "clear":
		return NewBuiltin("clear", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			l.Elts = []Object{}
			return None, nil
		}), nil
	case "copy":
		return NewBuiltin("copy", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return NewList(append([]Object{}, l.Elts...)...), nil
		}), nil
	case "index":
		return NewBuiltin("index", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) < 1 {
				return nil, Raisef(TypeErrorClass, "index() takes at least one argument")
			}
			for i, v := range l.Elts {
				if Equal(v, args[0]) {
					return NewInt(int64(i)), nil
				}
			}
			return nil, Raisef(ValueErrorClass, "%s is not in list", args[0].Repr())
		}), nil
	case "count":
		return NewBuiltin("count", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "count() takes exactly one argument (%d given)", len(args))
			}
			n := 0
			for _, v := range l.Elts {
				if Equal(v, args[0]) {
					n++
				}
			}
			return NewInt(int64(n)), nil
		}), nil
	case "reverse":
		return NewBuiltin("reverse", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			for i, j := 0, len(l.Elts)-1; i < j; i, j = i+1, j-1 {
				l.Elts[i], l.Elts[j] = l.Elts[j], l.Elts[i]
			}
			return None, nil
		}), nil
	case "sort":
		return NewBuiltin("sort", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			reverse := false
			if v, ok := kwargs["reverse"]; ok {
				reverse = v.Truthy()
			}
			var keyFn Object
			if v, ok := kwargs["key"]; ok {
				keyFn = v
			}
			keyOf := func(o Object) (Object, error) {
				if keyFn == nil || keyFn == None {
					return o, nil
				}
				return ctx.Call(keyFn, []Object{o}, nil)
			}
			var sortErr error
			sort.SliceStable(l.Elts, func(i, j int) bool {
				if sortErr != nil {
					return false
				}
				ki, err := keyOf(l.Elts[i])
				if err != nil {
					sortErr = err
					return false
				}
				kj, err := keyOf(l.Elts[j])
				if err != nil {
					sortErr = err
					return false
				}
				if reverse {
					less, err := Less(kj, ki)
					if err != nil {
						sortErr = err
						return false
					}
					return less
				}
				less, err := Less(ki, kj)
				if err != nil {
					sortErr = err
					return false
				}
				return less
			})
			if sortErr != nil {
				return nil, sortErr
			}
			return None, nil
		}), nil
	}
	return nil, attrError("list", name)
}

func tupleAttr(t *Tuple, name string) (Object, error) {
	switch name {
	case "index":
		return NewBuiltin("index", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) < 1 {
				return nil, Raisef(TypeErrorClass, "index() takes at least one argument")
			}
			for i, v := range t.Elts {
				if Equal(v, args[0]) {
					return NewInt(int64(i)), nil
				}
			}
			return nil, Raisef(ValueErrorClass, "%s is not in tuple", args[0].Repr())
		}), nil
	case "count":
		return NewBuiltin("count", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "count() takes exactly one argument (%d given)", len(args))
			}
			n := 0
			for _, v := range t.Elts {
				if Equal(v, args[0]) {
					n++
				}
			}
			return NewInt(int64(n)), nil
		}), nil
	}
	return nil, attrError("tuple", name)
}

func dictAttr(d *Dict, name string) (Object, error) {
	switch name {
	case "keys":
		return NewBuiltin("keys", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return NewList(append([]Object{}, d.Keys...)...), nil
		}), nil
	case "values":
		return NewBuiltin("values", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return NewList(append([]Object{}, d.Values...)...), nil
		}), nil
	case "items":
		return NewBuiltin("items", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			out := make([]Object, len(d.Keys))
			for i := range d.Keys {
				out[i] = NewTuple(d.Keys[i], d.Values[i])
			}
			return NewList(out...), nil
		}), nil
	case "get":
		return NewBuiltin("get", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) < 1 || len(args) > 2 {
				return nil, Raisef(TypeErrorClass, "get() takes 1 or 2 arguments")
			}
			if v, ok := d.Get(args[0]); ok {
				return v, nil
			}
			if len(args) == 2 {
				return args[1], nil
			}
			return None, nil
		}), nil
	case "pop":
		return NewBuiltin("pop", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) < 1 || len(args) > 2 {
				return nil, Raisef(TypeErrorClass, "pop() takes 1 or 2 arguments")
			}
			v, err := d.Delete(args[0])
			if err != nil {
				if len(args) == 2 {
					return args[1], nil
				}
				return nil, err
			}
			return v, nil
		}), nil
	case "update":
		return NewBuiltin("update", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) > 1 {
				return nil, Raisef(TypeErrorClass, "update() takes at most 1 positional argument")
			}
			if len(args) == 1 {
				if other, ok := args[0].(*Dict); ok {
					for i, k := range other.Keys {
						if err := d.Set(k, other.Values[i]); err != nil {
							return nil, err
						}
					}
				} else {
					it, err := Iterate(args[0])
					if err != nil {
						return nil, err
					}
					for {
						pair, err := it.Next()
						if err == ErrIterationDone {
							break
						}
						if err != nil {
							return nil, err
						}
						pit, err := Iterate(pair)
						if err != nil {
							return nil, err
						}
						k, err := pit.Next()
						if err != nil {
							return nil, Raisef(ValueErrorClass, "dictionary update sequence element has length 0")
						}
						v, err := pit.Next()
						if err != nil {
							return nil, Raisef(ValueErrorClass, "dictionary update sequence element has length 1")
						}
						if err := d.Set(k, v); err != nil {
							return nil, err
						}
					}
				}
			}
			for k, v := range kwargs {
				if err := d.Set(NewStr(k), v); err != nil {
					return nil, err
				}
			}
			return None, nil
		}), nil
	case "setdefault":
		return NewBuiltin("setdefault", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) < 1 || len(args) > 2 {
				return nil, Raisef(TypeErrorClass, "setdefault() takes 1 or 2 arguments")
			}
			if v, ok := d.Get(args[0]); ok {
				return v, nil
			}
			def := Object(None)
			if len(args) == 2 {
				def = args[1]
			}
			if err := d.Set(args[0], def); err != nil {
				return nil, err
			}
			return def, nil
		}), nil
	case "clear":
		return NewBuiltin("clear", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			d.Keys = []Object{}
			d.Values = []Object{}
			return None, nil
		}), nil
	case "copy":
		return NewBuiltin("copy", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			out := NewDict()
			for i, k := range d.Keys {
				_ = out.Set(k, d.Values[i])
			}
			return out, nil
		}), nil
	}
	return nil, attrError("dict", name)
}

func setAttr(s *Set, name string) (Object, error) {
	switch name {
	case "add":
		return NewBuiltin("add", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "add() takes exactly one argument (%d given)", len(args))
			}
			if err := s.Add(args[0]); err != nil {
				return nil, err
			}
			return None, nil
		}), nil
	case "discard":
		return NewBuiltin("discard", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "discard() takes exactly one argument (%d given)", len(args))
			}
			for i, v := range s.Elts {
				if Equal(v, args[0]) {
					s.Elts = append(s.Elts[:i], s.Elts[i+1:]...)
					break
				}
			}
			return None, nil
		}), nil
	case "remove":
		return NewBuiltin("remove", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "remove() takes exactly one argument (%d given)", len(args))
			}
			for i, v := range s.Elts {
				if Equal(v, args[0]) {
					s.Elts = append(s.Elts[:i], s.Elts[i+1:]...)
					return None, nil
				}
			}
			return nil, Raise(KeyError, args[0])
		}), nil
	case "clear":
		return NewBuiltin("clear", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			s.Elts = []Object{}
			return None, nil
		}), nil
	case "copy":
		return NewBuiltin("copy", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return NewSet(append([]Object{}, s.Elts...)...), nil
		}), nil
	}
	return nil, attrError("set", name)
}

func strAttr(s *Str, name string) (Object, error) {
	switch name {
	case "upper":
		return NewBuiltin("upper", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return NewStr(strings.ToUpper(s.V)), nil
		}), nil
	case "lower":
		return NewBuiltin("lower", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return NewStr(strings.ToLower(s.V)), nil
		}), nil
	case "strip":
		return NewBuiltin("strip", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) == 1 {
				c, ok := args[0].(*Str)
				if !ok {
					return nil, Raisef(TypeErrorClass, "strip() arg must be str")
				}
				return NewStr(strings.Trim(s.V, c.V)), nil
			}
			return NewStr(strings.TrimSpace(s.V)), nil
		}), nil
	case "lstrip":
		return NewBuiltin("lstrip", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) == 1 {
				c, ok := args[0].(*Str)
				if !ok {
					return nil, Raisef(TypeErrorClass, "lstrip() arg must be str")
				}
				return NewStr(strings.TrimLeft(s.V, c.V)), nil
			}
			return NewStr(strings.TrimLeft(s.V, " \t\n\r")), nil
		}), nil
	case "rstrip":
		return NewBuiltin("rstrip", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) == 1 {
				c, ok := args[0].(*Str)
				if !ok {
					return nil, Raisef(TypeErrorClass, "rstrip() arg must be str")
				}
				return NewStr(strings.TrimRight(s.V, c.V)), nil
			}
			return NewStr(strings.TrimRight(s.V, " \t\n\r")), nil
		}), nil
	case "split":
		return NewBuiltin("split", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			var parts []string
			if len(args) == 0 || args[0] == None {
				parts = strings.Fields(s.V)
			} else {
				sep, ok := args[0].(*Str)
				if !ok {
					return nil, Raisef(TypeErrorClass, "split() arg must be str")
				}
				parts = strings.Split(s.V, sep.V)
			}
			out := make([]Object, len(parts))
			for i, p := range parts {
				out[i] = NewStr(p)
			}
			return NewList(out...), nil
		}), nil
	case "join":
		return NewBuiltin("join", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "join() takes exactly one argument (%d given)", len(args))
			}
			it, err := Iterate(args[0])
			if err != nil {
				return nil, err
			}
			var parts []string
			for {
				v, err := it.Next()
				if err == ErrIterationDone {
					break
				}
				if err != nil {
					return nil, err
				}
				sv, ok := v.(*Str)
				if !ok {
					return nil, Raisef(TypeErrorClass, "sequence item: expected str instance, '%s' found", v.TypeName())
				}
				parts = append(parts, sv.V)
			}
			return NewStr(strings.Join(parts, s.V)), nil
		}), nil
	case "replace":
		return NewBuiltin("replace", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) < 2 || len(args) > 3 {
				return nil, Raisef(TypeErrorClass, "replace() takes 2 or 3 arguments")
			}
			old, ok1 := args[0].(*Str)
			nw, ok2 := args[1].(*Str)
			if !ok1 || !ok2 {
				return nil, Raisef(TypeErrorClass, "replace() arguments must be str")
			}
			n := -1
			if len(args) == 3 {
				v, ok := asInt(args[2])
				if !ok {
					return nil, Raisef(TypeErrorClass, "replace() count must be integer")
				}
				n = int(v)
			}
			return NewStr(strings.Replace(s.V, old.V, nw.V, n)), nil
		}), nil
	case "startswith":
		return NewBuiltin("startswith", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "startswith() takes exactly one argument")
			}
			p, ok := args[0].(*Str)
			if !ok {
				return nil, Raisef(TypeErrorClass, "startswith() arg must be str")
			}
			return BoolFrom(strings.HasPrefix(s.V, p.V)), nil
		}), nil
	case "endswith":
		return NewBuiltin("endswith", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "endswith() takes exactly one argument")
			}
			p, ok := args[0].(*Str)
			if !ok {
				return nil, Raisef(TypeErrorClass, "endswith() arg must be str")
			}
			return BoolFrom(strings.HasSuffix(s.V, p.V)), nil
		}), nil
	case "find":
		return NewBuiltin("find", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raisef(TypeErrorClass, "find() takes exactly one argument")
			}
			p, ok := args[0].(*Str)
			if !ok {
				return nil, Raisef(TypeErrorClass, "find() arg must be str")
			}
			return NewInt(int64(strings.Index(s.V, p.V))), nil
		}), nil
	case "format":
		return NewBuiltin("format", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return nil, Raisef(NotImplementedError, "str.format is not implemented")
		}), nil
	}
	return nil, attrError("str", name)
}
