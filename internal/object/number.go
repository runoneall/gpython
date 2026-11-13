package object

import (
	"math"
	"strings"
)

var opSymbols = map[string]string{
	"Add": "+", "Sub": "-", "Mult": "*", "Div": "/", "FloorDiv": "//",
	"Mod": "%", "Pow": "**", "MatMult": "@", "LShift": "<<", "RShift": ">>",
	"BitOr": "|", "BitXor": "^", "BitAnd": "&",
}

func numeric(o Object) (f float64, isInt bool, i int64, ok bool) {
	switch x := o.(type) {
	case *Int:
		return float64(x.V), true, x.V, true
	case *Float:
		return x.V, false, 0, true
	case Bool:
		v := int64(boolInt(x))
		return float64(v), true, v, true
	}
	return 0, false, 0, false
}

func typeError(op string, l, r Object) error {
	return Raisef(TypeErrorClass, "unsupported operand type(s) for %s: '%s' and '%s'", op, l.TypeName(), r.TypeName())
}

// BinaryOp implements the binary operators.
func BinaryOp(op string, l, r Object) (Object, error) {
	if op == "Div" {
		if d, ok := l.(Divider); ok {
			return d.Div(r)
		}
	}
	switch op {
	case "Add":
		return add(l, r)
	case "Sub", "Mult", "Div", "FloorDiv", "Mod", "Pow":
		return arithmetic(op, l, r)
	case "LShift", "RShift", "BitOr", "BitXor", "BitAnd":
		return bitwise(op, l, r)
	case "MatMult":
		return nil, Raisef(NotImplementedError, "matrix multiplication is not supported")
	}
	return nil, Raisef(RuntimeError, "unknown binary operator '%s'", op)
}

func add(l, r Object) (Object, error) {
	if a, ok := l.(*Str); ok {
		if b, ok := r.(*Str); ok {
			return NewStr(a.V + b.V), nil
		}
	}
	if a, ok := l.(*List); ok {
		if b, ok := r.(*List); ok {
			out := make([]Object, 0, len(a.Elts)+len(b.Elts))
			out = append(out, a.Elts...)
			out = append(out, b.Elts...)
			return NewList(out...), nil
		}
	}
	if a, ok := l.(*Tuple); ok {
		if b, ok := r.(*Tuple); ok {
			out := make([]Object, 0, len(a.Elts)+len(b.Elts))
			out = append(out, a.Elts...)
			out = append(out, b.Elts...)
			return NewTuple(out...), nil
		}
	}
	lf, li, lv, lok := numeric(l)
	rf, ri, rv, rok := numeric(r)
	if lok && rok {
		if li && ri {
			return NewInt(lv + rv), nil
		}
		return NewFloat(lf + rf), nil
	}
	return nil, typeError("+", l, r)
}

func arithmetic(op string, l, r Object) (Object, error) {
	if op == "Mod" {
		if a, ok := l.(*Str); ok {
			return PercentFormat(a.V, r)
		}
	}
	lf, li, lv, lok := numeric(l)
	rf, ri, rv, rok := numeric(r)
	if lok && rok {
		switch op {
		case "Sub":
			if li && ri {
				return NewInt(lv - rv), nil
			}
			return NewFloat(lf - rf), nil
		case "Mult":
			if li && ri {
				return NewInt(lv * rv), nil
			}
			return NewFloat(lf * rf), nil
		case "Div":
			if rv == 0 {
				return nil, Raisef(ZeroDivisionError, "division by zero")
			}
			return NewFloat(lf / rf), nil
		case "FloorDiv":
			if rv == 0 {
				return nil, Raisef(ZeroDivisionError, "integer division or modulo by zero")
			}
			if li && ri {
				return NewInt(floorDivInt(lv, rv)), nil
			}
			return NewFloat(math.Floor(lf / rf)), nil
		case "Mod":
			if rv == 0 {
				return nil, Raisef(ZeroDivisionError, "integer division or modulo by zero")
			}
			if li && ri {
				return NewInt(floorModInt(lv, rv)), nil
			}
			m := math.Mod(lf, rf)
			if m != 0 && (m < 0) != (rf < 0) {
				m += rf
			}
			return NewFloat(m), nil
		case "Pow":
			if li && ri && rv >= 0 {
				result := int64(1)
				base := lv
				exp := rv
				for exp > 0 {
					if exp&1 == 1 {
						result *= base
					}
					base *= base
					exp >>= 1
				}
				return NewInt(result), nil
			}
			return NewFloat(math.Pow(lf, rf)), nil
		}
	}
	if op == "Mult" {
		if a, ok := l.(*Str); ok {
			if n, ok := asInt(r); ok {
				return NewStr(repeatString(a.V, n)), nil
			}
		}
		if n, ok := asInt(l); ok {
			if b, ok := r.(*Str); ok {
				return NewStr(repeatString(b.V, n)), nil
			}
		}
		if a, ok := l.(*List); ok {
			if n, ok := asInt(r); ok {
				return repeatList(a, n), nil
			}
		}
		if n, ok := asInt(l); ok {
			if b, ok := r.(*List); ok {
				return repeatList(b, n), nil
			}
		}
		if a, ok := l.(*Tuple); ok {
			if n, ok := asInt(r); ok {
				return repeatTuple(a, n), nil
			}
		}
		if n, ok := asInt(l); ok {
			if b, ok := r.(*Tuple); ok {
				return repeatTuple(b, n), nil
			}
		}
	}
	return nil, typeError(opSymbols[op], l, r)
}

func floorDivInt(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func floorModInt(a, b int64) int64 {
	m := a % b
	if m != 0 && ((m < 0) != (b < 0)) {
		m += b
	}
	return m
}

func repeatString(s string, n int64) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, int(n))
}

func repeatList(l *List, n int64) *List {
	if n <= 0 {
		return NewList()
	}
	out := make([]Object, 0, len(l.Elts)*int(n))
	for i := int64(0); i < n; i++ {
		out = append(out, l.Elts...)
	}
	return NewList(out...)
}

func repeatTuple(t *Tuple, n int64) *Tuple {
	if n <= 0 {
		return NewTuple()
	}
	out := make([]Object, 0, len(t.Elts)*int(n))
	for i := int64(0); i < n; i++ {
		out = append(out, t.Elts...)
	}
	return NewTuple(out...)
}

func bitwise(op string, l, r Object) (Object, error) {
	a, ok1 := asInt(l)
	b, ok2 := asInt(r)
	if !ok1 || !ok2 {
		sym := opSymbols[op]
		return nil, typeError(sym, l, r)
	}
	switch op {
	case "LShift":
		if b < 0 {
			return nil, Raisef(ValueErrorClass, "negative shift count")
		}
		return NewInt(a << uint64(b)), nil
	case "RShift":
		if b < 0 {
			return nil, Raisef(ValueErrorClass, "negative shift count")
		}
		return NewInt(a >> uint64(b)), nil
	case "BitOr":
		return NewInt(a | b), nil
	case "BitXor":
		return NewInt(a ^ b), nil
	case "BitAnd":
		return NewInt(a & b), nil
	}
	return nil, Raisef(RuntimeError, "unknown bitwise operator '%s'", op)
}

// UnaryOp implements unary -x, +x and ~x.
func UnaryOp(op string, v Object) (Object, error) {
	switch x := v.(type) {
	case *Int:
		switch op {
		case "USub":
			return NewInt(-x.V), nil
		case "UAdd":
			return NewInt(x.V), nil
		case "Invert":
			return NewInt(^x.V), nil
		}
	case *Float:
		switch op {
		case "USub":
			return NewFloat(-x.V), nil
		case "UAdd":
			return NewFloat(x.V), nil
		}
	case Bool:
		switch op {
		case "USub":
			return NewInt(-int64(boolInt(x))), nil
		case "UAdd":
			return NewInt(int64(boolInt(x))), nil
		case "Invert":
			return NewInt(^int64(boolInt(x))), nil
		}
	}
	sym := map[string]string{"USub": "-", "UAdd": "+", "Invert": "~"}[op]
	return nil, Raisef(TypeErrorClass, "bad operand type for unary %s: '%s'", sym, v.TypeName())
}

// Compare implements comparisons, including `is` and `in`.
func Compare(op string, l, r Object) (Object, error) {
	switch op {
	case "Eq":
		return BoolFrom(Equal(l, r)), nil
	case "NotEq":
		return BoolFrom(!Equal(l, r)), nil
	case "Is":
		return BoolFrom(Is(l, r)), nil
	case "IsNot":
		return BoolFrom(!Is(l, r)), nil
	case "In":
		ok, err := Contains(r, l)
		return BoolFrom(ok), err
	case "NotIn":
		ok, err := Contains(r, l)
		return BoolFrom(!ok), err
	}
	c, err := compareOrder(l, r)
	if err != nil {
		return nil, err
	}
	switch op {
	case "Lt":
		return BoolFrom(c < 0), nil
	case "LtE":
		return BoolFrom(c <= 0), nil
	case "Gt":
		return BoolFrom(c > 0), nil
	case "GtE":
		return BoolFrom(c >= 0), nil
	}
	return nil, Raisef(RuntimeError, "unknown comparison operator '%s'", op)
}

func compareOrder(l, r Object) (int, error) {
	lf, _, lv, lok := numeric(l)
	rf, _, rv, rok := numeric(r)
	if lok && rok {
		_ = lv
		_ = rv
		switch {
		case lf < rf:
			return -1, nil
		case lf > rf:
			return 1, nil
		}
		return 0, nil
	}
	if a, ok := l.(*Str); ok {
		if b, ok := r.(*Str); ok {
			return strings.Compare(a.V, b.V), nil
		}
	}
	if a, ok := l.(*List); ok {
		if b, ok := r.(*List); ok {
			return compareSeq(a.Elts, b.Elts)
		}
	}
	if a, ok := l.(*Tuple); ok {
		if b, ok := r.(*Tuple); ok {
			return compareSeq(a.Elts, b.Elts)
		}
	}
	return 0, Raisef(TypeErrorClass, "'<' not supported between instances of '%s' and '%s'", l.TypeName(), r.TypeName())
}

func compareSeq(a, b []Object) (int, error) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if Equal(a[i], b[i]) {
			continue
		}
		return compareOrder(a[i], b[i])
	}
	switch {
	case len(a) < len(b):
		return -1, nil
	case len(a) > len(b):
		return 1, nil
	}
	return 0, nil
}

// Less reports whether a < b. Used by sorting.
func Less(a, b Object) (bool, error) {
	c, err := compareOrder(a, b)
	if err != nil {
		return false, err
	}
	return c < 0, nil
}

// Contains implements `item in container`.
func Contains(container, item Object) (bool, error) {
	switch c := container.(type) {
	case *Str:
		s, ok := item.(*Str)
		if !ok {
			return false, Raisef(TypeErrorClass, "'in <string>' requires string as left operand, not '%s'", item.TypeName())
		}
		return strings.Contains(c.V, s.V), nil
	case *List:
		for _, v := range c.Elts {
			if Equal(v, item) {
				return true, nil
			}
		}
		return false, nil
	case *Tuple:
		for _, v := range c.Elts {
			if Equal(v, item) {
				return true, nil
			}
		}
		return false, nil
	case *Dict:
		return c.Index(item) >= 0, nil
	case *Set:
		for _, v := range c.Elts {
			if Equal(v, item) {
				return true, nil
			}
		}
		return false, nil
	case *Range:
		i, ok := asInt(item)
		if !ok {
			return false, nil
		}
		if c.Step > 0 {
			return i >= c.Start && i < c.Stop && (i-c.Start)%c.Step == 0, nil
		}
		return i <= c.Start && i > c.Stop && (c.Start-i)%(-c.Step) == 0, nil
	case *Generator, *IterObject:
		it, err := Iterate(container)
		if err != nil {
			return false, err
		}
		for {
			v, err := it.Next()
			if err == ErrIterationDone {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if Equal(v, item) {
				return true, nil
			}
		}
	}
	return false, Raisef(TypeErrorClass, "argument of type '%s' is not iterable", container.TypeName())
}
