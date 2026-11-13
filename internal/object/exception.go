package object

import (
	"fmt"
	"strings"
)

// The builtin exception hierarchy. Names keep their Python spelling;
// when an error is printed the type name is lowercased (see PyError).
var (
	BaseExceptionClass  = newExceptionRoot("BaseException")
	ExceptionClass      = newExceptionClass("Exception", BaseExceptionClass)
	ArithmeticError     = newExceptionClass("ArithmeticError", ExceptionClass)
	AssertionError      = newExceptionClass("AssertionError", ExceptionClass)
	AttributeError      = newExceptionClass("AttributeError", ExceptionClass)
	ImportError         = newExceptionClass("ImportError", ExceptionClass)
	LookupError         = newExceptionClass("LookupError", ExceptionClass)
	NameError           = newExceptionClass("NameError", ExceptionClass)
	NotImplementedError = newExceptionClass("NotImplementedError", ExceptionClass)
	OSError             = newExceptionClass("OSError", ExceptionClass)
	RuntimeError        = newExceptionClass("RuntimeError", ExceptionClass)
	StopIteration       = newExceptionClass("StopIteration", ExceptionClass)
	SyntaxErrorClass    = newExceptionClass("SyntaxError", ExceptionClass)
	TypeErrorClass      = newExceptionClass("TypeError", ExceptionClass)
	ValueErrorClass     = newExceptionClass("ValueError", ExceptionClass)

	ZeroDivisionError   = newExceptionClass("ZeroDivisionError", ArithmeticError)
	OverflowError       = newExceptionClass("OverflowError", ArithmeticError)
	ModuleNotFoundError = newExceptionClass("ModuleNotFoundError", ImportError)
	IndexError          = newExceptionClass("IndexError", LookupError)
	KeyError            = newExceptionClass("KeyError", LookupError)
	KeyboardInterrupt   = newExceptionClass("KeyboardInterrupt", BaseExceptionClass)
	SystemExit          = newExceptionClass("SystemExit", BaseExceptionClass)
	RecursionError      = newExceptionClass("RecursionError", RuntimeError)
)

func newExceptionRoot(name string) *Class {
	c := NewClass(name, []*Class{ObjectClass}, "")
	c.IsException = true
	return c
}

func newExceptionClass(name string, base *Class) *Class {
	c := NewClass(name, []*Class{base}, "")
	c.IsException = true
	return c
}

// NewException creates an exception instance. args are stored on the
// instance under the conventional "args" attribute.
func NewException(class *Class, args ...Object) *Instance {
	inst := &Instance{Class: class, Dict: map[string]Object{}}
	if args == nil {
		args = []Object{}
	}
	inst.Dict["args"] = NewTuple(args...)
	return inst
}

// ExceptionArgs returns the positional arguments of an exception
// instance.
func ExceptionArgs(o Object) []Object {
	inst, ok := o.(*Instance)
	if !ok {
		return nil
	}
	if t, ok := inst.Dict["args"].(*Tuple); ok {
		return t.Elts
	}
	return nil
}

// PyError wraps a Python exception value as a Go error so that it can
// propagate through the evaluator with ordinary error returns.
type PyError struct {
	Value Object
	Trace []TraceFrame
}

// TraceFrame is one entry in a lowercase traceback.
type TraceFrame struct {
	File string
	Line int
	Name string
}

func (e *PyError) Error() string {
	inst, ok := e.Value.(*Instance)
	if !ok {
		return "exception"
	}
	name := strings.ToLower(inst.Class.Name)
	msg := inst.Str()
	if msg == "" {
		return name
	}
	return name + ": " + msg
}

// ExceptionName returns the lowercase name of the exception.
func ExceptionName(e *PyError) string {
	if inst, ok := e.Value.(*Instance); ok {
		return strings.ToLower(inst.Class.Name)
	}
	return "exception"
}

// Raise builds a PyError for class and args.
func Raise(class *Class, args ...Object) error {
	return &PyError{Value: NewException(class, args...)}
}

// Raisef builds a PyError with a formatted, already-lowercase message.
func Raisef(class *Class, format string, args ...any) error {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return Raise(class, NewStr(msg))
}
