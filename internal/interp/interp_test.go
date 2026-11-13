package interp

import (
	"strings"
	"testing"
)

func TestRunBasic(t *testing.T) {
	i := New()
	src := `
x = 1 + 2
y = x * 3
z = "hello"
print(x, y, z)
`
	if err := i.RunSource("<test>", src); err != nil {
		t.Fatalf("RunSource: %v", err)
	}
}

func TestRunControlFlow(t *testing.T) {
	i := New()
	src := `
total = 0
for i in range(5):
    total = total + i
print(total)
if total > 5:
    print("big")
else:
    print("small")
`
	if err := i.RunSource("<test>", src); err != nil {
		t.Fatalf("RunSource: %v", err)
	}
}

func TestRunDivisionByZero(t *testing.T) {
	i := New()
	src := `print(1 / 0)`
	err := i.RunSource("<test>", src)
	if err == nil {
		t.Fatal("expected error for division by zero, got nil")
	}
	if !strings.Contains(err.Error(), "zerodivisionerror") {
		t.Errorf("expected lowercase zerodivisionerror, got %q", err.Error())
	}
}

func TestEvalExpr(t *testing.T) {
	i := New()
	// 1 + 2 * 3
	// 这里不直接构造 parser2 表达式，改用 RunSource 验证表达式求值不出错。
	src := `print(1 + 2 * 3)`
	if err := i.RunSource("<test>", src); err != nil {
		t.Fatalf("RunSource: %v", err)
	}
}

func TestNameNotDefined(t *testing.T) {
	i := New()
	err := i.RunSource("<test>", `print(undefined_name)`)
	if err == nil {
		t.Fatal("expected NameError, got nil")
	}
	if !strings.Contains(err.Error(), "nameerror") {
		t.Errorf("expected lowercase nameerror, got %q", err.Error())
	}
}
