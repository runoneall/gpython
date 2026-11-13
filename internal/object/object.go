// Package object implements the runtime values used by the gpython
// interpreter. It has no dependency on the evaluator so that the
// standard library packages can build values without an import cycle.
package object

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Object is the interface implemented by every runtime value.
type Object interface {
	TypeName() string
	Repr() string
	Str() string
	Truthy() bool
}

// NoneType is the type of Python's None.
type NoneType struct{}

// None is the single None value.
var None = &NoneType{}

func (*NoneType) TypeName() string { return "nonetype" }
func (*NoneType) Repr() string     { return "None" }
func (*NoneType) Str() string      { return "None" }
func (*NoneType) Truthy() bool     { return false }

// EllipsisType is the type of the `...` literal.
type EllipsisType struct{}

// Ellipsis is the single `...` value.
var Ellipsis = &EllipsisType{}

func (*EllipsisType) TypeName() string { return "ellipsis" }
func (*EllipsisType) Repr() string     { return "Ellipsis" }
func (*EllipsisType) Str() string      { return "Ellipsis" }
func (*EllipsisType) Truthy() bool     { return true }

// Bool is Python's bool. It is kept separate from Int so that repr and
// formatting match Python.
type Bool bool

var (
	True  = Bool(true)
	False = Bool(false)
)

func BoolFrom(b bool) Bool {
	if b {
		return True
	}
	return False
}

func (b Bool) TypeName() string { return "bool" }
func (b Bool) Repr() string {
	if b {
		return "True"
	}
	return "False"
}
func (b Bool) Str() string  { return b.Repr() }
func (b Bool) Truthy() bool { return bool(b) }

// Int is a 64-bit signed integer. gpython deliberately does not
// implement arbitrary precision integers.
type Int struct{ V int64 }

func NewInt(v int64) *Int { return &Int{V: v} }

func (*Int) TypeName() string { return "int" }
func (i *Int) Repr() string   { return strconv.FormatInt(i.V, 10) }
func (i *Int) Str() string    { return i.Repr() }
func (i *Int) Truthy() bool   { return i.V != 0 }

// Float is a 64-bit floating point number.
type Float struct{ V float64 }

func NewFloat(v float64) *Float { return &Float{V: v} }

func (*Float) TypeName() string { return "float" }
func (f *Float) Repr() string   { return FormatFloat(f.V) }
func (f *Float) Str() string    { return f.Repr() }
func (f *Float) Truthy() bool   { return f.V != 0 }

// FormatFloat renders a float the way CPython's repr does for the
// magnitudes used by normal programs.
func FormatFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// Str is a Python string.
type Str struct{ V string }

func NewStr(s string) *Str { return &Str{V: s} }

func (*Str) TypeName() string { return "str" }
func (s *Str) Repr() string   { return quoteString(s.V) }
func (s *Str) Str() string    { return s.V }
func (s *Str) Truthy() bool   { return s.V != "" }

func quoteString(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if byte(r) == quote && r < 128 {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 32 || r == 127 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// List is a mutable sequence.
type List struct{ Elts []Object }

func NewList(elts ...Object) *List {
	if elts == nil {
		elts = []Object{}
	}
	return &List{Elts: elts}
}

func (*List) TypeName() string { return "list" }
func (l *List) Repr() string   { return "[" + joinRepr(l.Elts) + "]" }
func (l *List) Str() string    { return l.Repr() }
func (l *List) Truthy() bool   { return len(l.Elts) > 0 }

// Tuple is an immutable sequence.
type Tuple struct{ Elts []Object }

func NewTuple(elts ...Object) *Tuple {
	if elts == nil {
		elts = []Object{}
	}
	return &Tuple{Elts: elts}
}

func (*Tuple) TypeName() string { return "tuple" }
func (t *Tuple) Repr() string {
	if len(t.Elts) == 1 {
		return "(" + t.Elts[0].Repr() + ",)"
	}
	return "(" + joinRepr(t.Elts) + ")"
}
func (t *Tuple) Str() string  { return t.Repr() }
func (t *Tuple) Truthy() bool { return len(t.Elts) > 0 }

// Set is an insertion-ordered set. CPython's set ordering is hash
// based; preserving insertion order gives deterministic output while
// still supporting the operations gpython needs.
type Set struct{ Elts []Object }

func NewSet(elts ...Object) *Set {
	return &Set{Elts: elts}
}

func (*Set) TypeName() string { return "set" }
func (s *Set) Repr() string {
	if len(s.Elts) == 0 {
		return "set()"
	}
	return "{" + joinRepr(s.Elts) + "}"
}
func (s *Set) Str() string  { return s.Repr() }
func (s *Set) Truthy() bool { return len(s.Elts) > 0 }

// Add inserts v if it is not already present.
func (s *Set) Add(v Object) error {
	if !Hashable(v) {
		return Raisef(TypeErrorClass, "unhashable type: '%s'", v.TypeName())
	}
	for _, e := range s.Elts {
		if Equal(e, v) {
			return nil
		}
	}
	s.Elts = append(s.Elts, v)
	return nil
}

// Dict is an insertion-ordered mapping. Lookups are linear because
// gpython favours a small, dependency-free implementation.
type Dict struct {
	Keys   []Object
	Values []Object
}

func NewDict() *Dict { return &Dict{Keys: []Object{}, Values: []Object{}} }

func (*Dict) TypeName() string { return "dict" }
func (d *Dict) Repr() string {
	parts := make([]string, len(d.Keys))
	for i, k := range d.Keys {
		parts[i] = k.Repr() + ": " + d.Values[i].Repr()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
func (d *Dict) Str() string  { return d.Repr() }
func (d *Dict) Truthy() bool { return len(d.Keys) > 0 }

// Index returns the position of key, or -1.
func (d *Dict) Index(key Object) int {
	for i, k := range d.Keys {
		if Equal(k, key) {
			return i
		}
	}
	return -1
}

func (d *Dict) Get(key Object) (Object, bool) {
	if i := d.Index(key); i >= 0 {
		return d.Values[i], true
	}
	return nil, false
}

func (d *Dict) Set(key, value Object) error {
	if !Hashable(key) {
		return Raisef(TypeErrorClass, "unhashable type: '%s'", key.TypeName())
	}
	if i := d.Index(key); i >= 0 {
		d.Values[i] = value
		return nil
	}
	d.Keys = append(d.Keys, key)
	d.Values = append(d.Values, value)
	return nil
}

func (d *Dict) Delete(key Object) (Object, error) {
	if i := d.Index(key); i >= 0 {
		v := d.Values[i]
		d.Keys = append(d.Keys[:i], d.Keys[i+1:]...)
		d.Values = append(d.Values[:i], d.Values[i+1:]...)
		return v, nil
	}
	return nil, Raise(KeyError, key)
}

// Range is the result of the range() builtin.
type Range struct {
	Start int64
	Stop  int64
	Step  int64
}

func NewRange(start, stop, step int64) *Range {
	return &Range{Start: start, Stop: stop, Step: step}
}

func (*Range) TypeName() string { return "range" }
func (r *Range) Repr() string {
	if r.Step == 1 {
		return fmt.Sprintf("range(%d, %d)", r.Start, r.Stop)
	}
	return fmt.Sprintf("range(%d, %d, %d)", r.Start, r.Stop, r.Step)
}
func (r *Range) Str() string  { return r.Repr() }
func (r *Range) Truthy() bool { return r.Len() > 0 }

// Len reports the number of values in the range.
func (r *Range) Len() int64 {
	if r.Step > 0 {
		if r.Start >= r.Stop {
			return 0
		}
		return (r.Stop - r.Start + r.Step - 1) / r.Step
	}
	if r.Start <= r.Stop {
		return 0
	}
	return (r.Start - r.Stop - r.Step - 1) / (-r.Step)
}

// Module is a Python module object.
type Module struct {
	Name      string
	Dict      map[string]Object
	IsPackage bool
	File      string
	Package   string
	Dir       string
}

func NewModule(name string) *Module {
	return &Module{Name: name, Dict: map[string]Object{}}
}

func (*Module) TypeName() string { return "module" }
func (m *Module) Repr() string   { return "<module '" + m.Name + "'>" }
func (m *Module) Str() string    { return m.Repr() }
func (*Module) Truthy() bool     { return true }

// Get returns a module attribute.
func (m *Module) Get(name string) (Object, bool) {
	v, ok := m.Dict[name]
	return v, ok
}

// Set stores a module attribute.
func (m *Module) Set(name string, v Object) { m.Dict[name] = v }

func joinRepr(elts []Object) string {
	parts := make([]string, len(elts))
	for i, e := range elts {
		parts[i] = e.Repr()
	}
	return strings.Join(parts, ", ")
}

// Hashable reports whether a value may be used as a dict key or set
// member.
func Hashable(o Object) bool {
	switch x := o.(type) {
	case *List, *Dict, *Set:
		return false
	case *Tuple:
		for _, e := range x.Elts {
			if !Hashable(e) {
				return false
			}
		}
	}
	return true
}

// Equal implements Python == for the supported value types.
func Equal(a, b Object) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch x := a.(type) {
	case *NoneType:
		_, ok := b.(*NoneType)
		return ok
	case Bool:
		switch y := b.(type) {
		case Bool:
			return x == y
		case *Int:
			return int64(boolInt(x)) == y.V
		}
		return false
	case *Int:
		switch y := b.(type) {
		case *Int:
			return x.V == y.V
		case *Float:
			return float64(x.V) == y.V
		case Bool:
			return x.V == int64(boolInt(y))
		}
		return false
	case *Float:
		switch y := b.(type) {
		case *Float:
			return x.V == y.V
		case *Int:
			return x.V == float64(y.V)
		case Bool:
			return x.V == float64(boolInt(y))
		}
		return false
	case *Str:
		y, ok := b.(*Str)
		return ok && x.V == y.V
	case *List:
		y, ok := b.(*List)
		if !ok || len(x.Elts) != len(y.Elts) {
			return false
		}
		for i := range x.Elts {
			if !Equal(x.Elts[i], y.Elts[i]) {
				return false
			}
		}
		return true
	case *Tuple:
		y, ok := b.(*Tuple)
		if !ok || len(x.Elts) != len(y.Elts) {
			return false
		}
		for i := range x.Elts {
			if !Equal(x.Elts[i], y.Elts[i]) {
				return false
			}
		}
		return true
	case *Dict:
		y, ok := b.(*Dict)
		if !ok || len(x.Keys) != len(y.Keys) {
			return false
		}
		for i, k := range x.Keys {
			v, found := y.Get(k)
			if !found || !Equal(x.Values[i], v) {
				return false
			}
		}
		return true
	case *Set:
		y, ok := b.(*Set)
		if !ok || len(x.Elts) != len(y.Elts) {
			return false
		}
		for _, e := range x.Elts {
			found := false
			for _, f := range y.Elts {
				if Equal(e, f) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	// Remaining values are pointers; identity is the closest
	// equivalent to Python's default equality.
	return Is(a, b)
}

// Is implements Python's `is` operator.
func Is(a, b Object) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch x := a.(type) {
	case *NoneType:
		_, ok := b.(*NoneType)
		return ok
	case Bool:
		y, ok := b.(Bool)
		return ok && x == y
	}
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	if va.Type() != vb.Type() || va.Kind() != reflect.Ptr {
		return false
	}
	return va.Pointer() == vb.Pointer()
}

func boolInt(b Bool) int {
	if b {
		return 1
	}
	return 0
}
