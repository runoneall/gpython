package object

import (
	"testing"
)

func TestNoneRepr(t *testing.T) {
	if None.Repr() != "None" {
		t.Errorf("None.Repr() = %q, want %q", None.Repr(), "None")
	}
	if None.Truthy() {
		t.Error("None.Truthy() should be false")
	}
}

func TestBool(t *testing.T) {
	if True.Repr() != "True" {
		t.Errorf("True.Repr() = %q", True.Repr())
	}
	if False.Repr() != "False" {
		t.Errorf("False.Repr() = %q", False.Repr())
	}
	if !True.Truthy() {
		t.Error("True.Truthy() should be true")
	}
	if False.Truthy() {
		t.Error("False.Truthy() should be false")
	}
}

func TestInt(t *testing.T) {
	i := NewInt(42)
	if i.Repr() != "42" {
		t.Errorf("NewInt(42).Repr() = %q", i.Repr())
	}
	if !i.Truthy() {
		t.Error("NewInt(42).Truthy() should be true")
	}
	if NewInt(0).Truthy() {
		t.Error("NewInt(0).Truthy() should be false")
	}
	if i.TypeName() != "int" {
		t.Errorf("TypeName() = %q", i.TypeName())
	}
}

func TestFloat(t *testing.T) {
	f := NewFloat(3.5)
	if f.Repr() != "3.5" {
		t.Errorf("NewFloat(3.5).Repr() = %q", f.Repr())
	}
	if FormatFloat(1.0) != "1.0" {
		t.Errorf("FormatFloat(1.0) = %q", FormatFloat(1.0))
	}
	if FormatFloat(3.141592653589793) != "3.141592653589793" {
		t.Errorf("FormatFloat(pi) = %q", FormatFloat(3.141592653589793))
	}
}

func TestStr(t *testing.T) {
	s := NewStr("hello")
	if s.Str() != "hello" {
		t.Errorf("Str() = %q", s.Str())
	}
	if s.Repr() != "'hello'" {
		t.Errorf("Repr() = %q", s.Repr())
	}
	if !s.Truthy() {
		t.Error("non-empty str should be truthy")
	}
	if NewStr("").Truthy() {
		t.Error("empty str should be falsy")
	}
}

func TestListTuple(t *testing.T) {
	l := NewList(NewInt(1), NewInt(2), NewInt(3))
	if l.Repr() != "[1, 2, 3]" {
		t.Errorf("list Repr() = %q", l.Repr())
	}
	if !l.Truthy() {
		t.Error("non-empty list should be truthy")
	}
	if NewList().Truthy() {
		t.Error("empty list should be falsy")
	}

	tp := NewTuple(NewInt(1))
	if tp.Repr() != "(1,)" {
		t.Errorf("single tuple Repr() = %q", tp.Repr())
	}
	tp2 := NewTuple(NewInt(1), NewInt(2))
	if tp2.Repr() != "(1, 2)" {
		t.Errorf("tuple Repr() = %q", tp2.Repr())
	}
}

func TestDict(t *testing.T) {
	d := NewDict()
	if err := d.Set(NewStr("a"), NewInt(1)); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := d.Set(NewStr("b"), NewInt(2)); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if d.Repr() != "{'a': 1, 'b': 2}" {
		t.Errorf("dict Repr() = %q", d.Repr())
	}
	v, ok := d.Get(NewStr("a"))
	if !ok || v.Repr() != "1" {
		t.Errorf("Get(a) = %v, %v", v, ok)
	}
	if _, ok := d.Get(NewStr("missing")); ok {
		t.Error("Get(missing) should not be found")
	}
	if _, err := d.Delete(NewStr("a")); err != nil {
		t.Errorf("Delete(a): %v", err)
	}
	if d.Index(NewStr("a")) != -1 {
		t.Error("Index(a) should be -1 after delete")
	}
	if d.Truthy() == false {
		t.Error("dict with one item should be truthy")
	}
}

func TestSet(t *testing.T) {
	s := NewSet()
	if err := s.Add(NewInt(1)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add(NewInt(1)); err != nil {
		t.Fatalf("Add dup: %v", err)
	}
	if err := s.Add(NewInt(2)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if s.Repr() != "{1, 2}" {
		t.Errorf("set Repr() = %q", s.Repr())
	}
	if NewSet().Truthy() {
		t.Error("empty set should be falsy")
	}
}

func TestEqual(t *testing.T) {
	if !Equal(NewInt(1), NewInt(1)) {
		t.Error("1 == 1 should be true")
	}
	if Equal(NewInt(1), NewInt(2)) {
		t.Error("1 == 2 should be false")
	}
	if !Equal(NewStr("x"), NewStr("x")) {
		t.Error("\"x\" == \"x\" should be true")
	}
	if !Equal(NewList(NewInt(1)), NewList(NewInt(1))) {
		t.Error("[1] == [1] should be true")
	}
	if Equal(None, NewInt(0)) {
		t.Error("None == 0 should be false")
	}
	if !Equal(True, NewInt(1)) {
		t.Error("True == 1 should be true")
	}
}

func TestHashable(t *testing.T) {
	if Hashable(NewList(NewInt(1))) {
		t.Error("list should not be hashable")
	}
	if Hashable(NewDict()) {
		t.Error("dict should not be hashable")
	}
	if !Hashable(NewTuple(NewInt(1))) {
		t.Error("tuple of ints should be hashable")
	}
	if !Hashable(NewInt(1)) {
		t.Error("int should be hashable")
	}
}
