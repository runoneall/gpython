package builtins

import (
	"testing"

	"gpython/internal/object"
)

func TestScopeContainsBuiltins(t *testing.T) {
	s := Scope()
	want := []string{
		"None", "True", "False", "Ellipsis",
		"print", "len", "int", "str", "float", "bool",
		"list", "tuple", "dict", "set", "range",
		"abs", "min", "max", "sum", "sorted",
		"isinstance", "issubclass", "repr", "type",
	}
	for _, name := range want {
		if _, ok := s[name]; !ok {
			t.Errorf("Scope() missing builtin %q", name)
		}
	}
}

func TestScopeValuesAreObjects(t *testing.T) {
	s := Scope()
	none, ok := s["None"]
	if !ok {
		t.Fatal("Scope() missing None")
	}
	if none != object.None {
		t.Error("Scope() None is not object.None")
	}
	if none.Repr() != "None" {
		t.Errorf("None.Repr() = %q", none.Repr())
	}
}

func TestExceptionsRegistered(t *testing.T) {
	s := Scope()
	want := []string{"Exception", "ValueError", "TypeError", "NameError", "ZeroDivisionError", "KeyError", "AttributeError"}
	for _, name := range want {
		if _, ok := s[name]; !ok {
			t.Errorf("Scope() missing exception %q", name)
		}
	}
}
