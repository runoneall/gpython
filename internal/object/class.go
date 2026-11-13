package object

import "strings"

// ObjectClass is the root of every class hierarchy.
var ObjectClass = func() *Class {
	c := &Class{Name: "object", Dict: map[string]Object{}}
	c.MRO = []*Class{c}
	return c
}()

// Classes representing the builtin value types. They make isinstance,
// issubclass and type() behave sensibly without a metaclass protocol.
var (
	TypeClass      = NewClass("type", []*Class{ObjectClass}, "type")
	NoneClass      = NewClass("NoneType", []*Class{ObjectClass}, "nonetype")
	BoolClass      = NewClass("bool", []*Class{ObjectClass}, "bool")
	IntClass       = NewClass("int", []*Class{ObjectClass}, "int")
	FloatClass     = NewClass("float", []*Class{ObjectClass}, "float")
	StrClass       = NewClass("str", []*Class{ObjectClass}, "str")
	ListClass      = NewClass("list", []*Class{ObjectClass}, "list")
	TupleClass     = NewClass("tuple", []*Class{ObjectClass}, "tuple")
	DictClass      = NewClass("dict", []*Class{ObjectClass}, "dict")
	SetClass       = NewClass("set", []*Class{ObjectClass}, "set")
	RangeClass     = NewClass("range", []*Class{ObjectClass}, "range")
	FunctionClass  = NewClass("function", []*Class{ObjectClass}, "function")
	BuiltinClass   = NewClass("builtin_function_or_method", []*Class{ObjectClass}, "builtin")
	MethodClass    = NewClass("method", []*Class{ObjectClass}, "method")
	GeneratorClass = NewClass("generator", []*Class{ObjectClass}, "generator")
	ModuleClass    = NewClass("module", []*Class{ObjectClass}, "module")
	IteratorClass  = NewClass("iterator", []*Class{ObjectClass}, "iterator")
	SuperClass     = NewClass("super", []*Class{ObjectClass}, "super")
	SliceClass     = NewClass("slice", []*Class{ObjectClass}, "slice")
)

// Class is a Python class object.
type Class struct {
	Name        string
	Bases       []*Class
	MRO         []*Class
	Dict        map[string]Object
	IsException bool
	BuiltinKind string
}

// NewClass builds a class and computes its MRO. A class with no bases
// implicitly inherits from object.
func NewClass(name string, bases []*Class, builtinKind string) *Class {
	if len(bases) == 0 {
		bases = []*Class{ObjectClass}
	}
	c := &Class{Name: name, Bases: bases, Dict: map[string]Object{}, BuiltinKind: builtinKind}
	c.MRO = computeMRO(c, bases)
	for _, b := range bases {
		if b.IsException {
			c.IsException = true
		}
	}
	return c
}

func computeMRO(c *Class, bases []*Class) []*Class {
	seqs := make([][]*Class, 0, len(bases)+1)
	for _, b := range bases {
		seq := append([]*Class{b}, b.MRO...)
		seqs = append(seqs, seq)
	}
	seqs = append(seqs, append([]*Class{}, bases...))

	result := []*Class{c}
	for {
		cand := (*Class)(nil)
		for _, seq := range seqs {
			if len(seq) == 0 {
				continue
			}
			head := seq[0]
			inTail := false
			for _, other := range seqs {
				if len(other) < 2 {
					continue
				}
				for _, x := range other[1:] {
					if x == head {
						inTail = true
					}
				}
			}
			if !inTail {
				cand = head
				break
			}
		}
		if cand == nil {
			// Inconsistent hierarchy: fall back to source order so
			// that programs still run.
			for _, seq := range seqs {
				if len(seq) > 0 {
					cand = seq[0]
					break
				}
			}
		}
		if cand == nil {
			break
		}
		already := false
		for _, x := range result {
			if x == cand {
				already = true
				break
			}
		}
		if !already {
			result = append(result, cand)
		}
		removed := false
		for i, seq := range seqs {
			if len(seq) > 0 && seq[0] == cand {
				seqs[i] = seq[1:]
				removed = true
			}
		}
		if !removed {
			break
		}
	}
	return result
}

func (*Class) TypeName() string { return "type" }
func (c *Class) Repr() string   { return "<class '" + c.Name + "'>" }
func (c *Class) Str() string    { return c.Repr() }
func (*Class) Truthy() bool     { return true }

// Lookup searches the class and its MRO for name.
func (c *Class) Lookup(name string) (Object, bool) {
	for _, base := range c.MRO {
		if v, ok := base.Dict[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// Instance is an instance of a user-defined (or exception) class.
type Instance struct {
	Class *Class
	Dict  map[string]Object
}

func NewInstance(c *Class) *Instance {
	return &Instance{Class: c, Dict: map[string]Object{}}
}

func (i *Instance) TypeName() string { return strings.ToLower(i.Class.Name) }

func (i *Instance) Repr() string {
	if i.Class.IsException {
		return i.Class.Name + "(" + joinRepr(ExceptionArgs(i)) + ")"
	}
	return "<" + i.Class.Name + " object>"
}

func (i *Instance) Str() string {
	if i.Class.IsException {
		args := ExceptionArgs(i)
		if len(args) == 0 {
			return ""
		}
		if i.Class == KeyError {
			return args[0].Repr()
		}
		if len(args) == 1 {
			return args[0].Str()
		}
		return NewTuple(args...).Repr()
	}
	return i.Repr()
}

func (*Instance) Truthy() bool { return true }

// Super is the object returned by super().
type Super struct {
	Class *Class
	Self  Object
}

func (*Super) TypeName() string { return "super" }
func (s *Super) Repr() string   { return "<super: " + s.Class.Name + ">" }
func (s *Super) Str() string    { return s.Repr() }
func (*Super) Truthy() bool     { return true }

// ClassOf returns the type object for a value.
func ClassOf(o Object) *Class {
	switch x := o.(type) {
	case *NoneType:
		return NoneClass
	case Bool:
		return BoolClass
	case *Int:
		return IntClass
	case *Float:
		return FloatClass
	case *Str:
		return StrClass
	case *List:
		return ListClass
	case *Tuple:
		return TupleClass
	case *Dict:
		return DictClass
	case *Set:
		return SetClass
	case *Range:
		return RangeClass
	case *Module:
		return ModuleClass
	case *Instance:
		return x.Class
	case *Class:
		return TypeClass
	case *Function:
		return FunctionClass
	case *BoundMethod:
		return MethodClass
	case *Builtin:
		return BuiltinClass
	case *Generator:
		return GeneratorClass
	case *IterObject:
		return IteratorClass
	case *Super:
		return SuperClass
	case *Slice:
		return SliceClass
	}
	return ObjectClass
}

// IsInstance implements isinstance().
func IsInstance(o Object, classinfo Object) (bool, error) {
	switch c := classinfo.(type) {
	case *Class:
		oc := ClassOf(o)
		for _, x := range oc.MRO {
			if x == c {
				return true, nil
			}
		}
		return false, nil
	case *Tuple:
		for _, e := range c.Elts {
			ok, err := IsInstance(o, e)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, Raisef(TypeErrorClass, "isinstance() arg 2 must be a type or tuple of types")
	}
}

// IsSubclass implements issubclass().
func IsSubclass(sub Object, classinfo Object) (bool, error) {
	sc, ok := sub.(*Class)
	if !ok {
		return false, Raisef(TypeErrorClass, "issubclass() arg 1 must be a class")
	}
	switch c := classinfo.(type) {
	case *Class:
		for _, x := range sc.MRO {
			if x == c {
				return true, nil
			}
		}
		return false, nil
	case *Tuple:
		for _, e := range c.Elts {
			ok, err := IsSubclass(sc, e)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, Raisef(TypeErrorClass, "issubclass() arg 2 must be a class or tuple of classes")
	}
}

// AttributeProvider lets packages outside object expose attributes on
// custom values (used by pathlib).
type AttributeProvider interface {
	GetAttr(name string) (Object, error)
	SetAttr(name string, value Object) error
}

// Callable lets packages outside object expose callable values.
type Callable interface {
	Call(ctx Caller, args []Object, kwargs map[string]Object) (Object, error)
}

// Divider lets packages outside object implement the `/` operator.
type Divider interface {
	Div(other Object) (Object, error)
}

func bind(v Object, self Object) Object {
	if f, ok := v.(*Function); ok {
		return &BoundMethod{Fn: f, Self: self}
	}
	return v
}

func attrError(typeName, attr string) error {
	return Raisef(AttributeError, "'%s' object has no attribute '%s'", typeName, attr)
}

// GetAttr implements attribute access.
func GetAttr(o Object, name string) (Object, error) {
	switch x := o.(type) {
	case *Module:
		if v, ok := x.Dict[name]; ok {
			return v, nil
		}
		return nil, Raisef(AttributeError, "module '%s' has no attribute '%s'", x.Name, name)
	case *Instance:
		if v, ok := x.Dict[name]; ok {
			return v, nil
		}
		if v, ok := x.Class.Lookup(name); ok {
			return bind(v, x), nil
		}
		return nil, attrError(x.TypeName(), name)
	case *Class:
		if name == "__name__" {
			return NewStr(x.Name), nil
		}
		if v, ok := x.Lookup(name); ok {
			return v, nil
		}
		return nil, attrError("type", name)
	case *Super:
		seen := false
		for _, c := range x.Class.MRO {
			if c == x.Class {
				seen = true
				continue
			}
			if !seen {
				continue
			}
			if v, ok := c.Dict[name]; ok {
				return bind(v, x.Self), nil
			}
		}
		return nil, attrError("super", name)
	case *List:
		return listAttr(x, name)
	case *Tuple:
		return tupleAttr(x, name)
	case *Dict:
		return dictAttr(x, name)
	case *Set:
		return setAttr(x, name)
	case *Str:
		return strAttr(x, name)
	case *Generator:
		return generatorAttr(x, name)
	case *Function:
		if name == "__name__" {
			return NewStr(x.Name), nil
		}
	case *Builtin:
		if name == "__name__" {
			return NewStr(x.Name), nil
		}
	case AttributeProvider:
		return x.GetAttr(name)
	}
	return nil, attrError(o.TypeName(), name)
}

// SetAttr implements attribute assignment.
func SetAttr(o Object, name string, value Object) error {
	switch x := o.(type) {
	case *Instance:
		x.Dict[name] = value
		return nil
	case *Class:
		x.Dict[name] = value
		return nil
	case *Module:
		x.Dict[name] = value
		return nil
	case AttributeProvider:
		return x.SetAttr(name, value)
	}
	return Raisef(AttributeError, "'%s' object attribute '%s' is read-only", o.TypeName(), name)
}
