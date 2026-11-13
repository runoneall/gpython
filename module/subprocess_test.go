package module

import (
	"testing"

	"gpython/internal/object"
)

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgsToStrings(t *testing.T) {
	cases := []struct {
		in   []object.Object
		want []string
	}{
		{[]object.Object{object.NewStr("echo"), object.NewStr("hi")}, []string{"echo", "hi"}},
		{[]object.Object{object.NewInt(42)}, []string{"42"}},
		{[]object.Object{object.NewFloat(2.5)}, []string{"2.5"}},
		{[]object.Object{object.True}, []string{"True"}},
		{[]object.Object{object.False}, []string{"False"}},
	}
	for _, c := range cases {
		got, err := argsToStrings(c.in)
		if err != nil {
			t.Fatalf("argsToStrings(%v): unexpected error: %v", c.in, err)
		}
		if len(got) != len(c.want) {
			t.Fatalf("argsToStrings(%v) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("argsToStrings(%v)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}

	// Empty args errors.
	if _, err := argsToStrings(nil); err == nil {
		t.Fatal("expected error for empty args")
	}
	// Non-str/number errors.
	if _, err := argsToStrings([]object.Object{object.None}); err == nil {
		t.Fatal("expected error for non-str/number arg")
	}
}

func TestArgsRepr(t *testing.T) {
	got := argsRepr([]object.Object{object.NewStr("a"), object.NewInt(1)})
	if got != "['a', 1]" {
		t.Fatalf("argsRepr = %q, want ['a', 1]", got)
	}
}

func TestCompletedProcess(t *testing.T) {
	cp := NewCompletedProcess([]object.Object{object.NewStr("echo")}, 0, "out\n", "err\n")
	if cp.TypeName() != "completedprocess" {
		t.Fatalf("TypeName = %q", cp.TypeName())
	}
	if cp.Repr() == "" || cp.Str() == "" {
		t.Fatal("empty repr/str")
	}
	if !cp.Truthy() {
		t.Fatal("expected truthy")
	}

	rc, err := cp.GetAttr("returncode")
	mustNoErr(t, err)
	if ri, ok := rc.(*object.Int); !ok || ri.V != 0 {
		t.Fatalf("returncode = %v (%v)", rc, err)
	}
	stdout, err := cp.GetAttr("stdout")
	mustNoErr(t, err)
	if ss, ok := stdout.(*object.Str); !ok || ss.V != "out\n" {
		t.Fatalf("stdout = %v (%v)", stdout, err)
	}
	stderr, err := cp.GetAttr("stderr")
	mustNoErr(t, err)
	if se, ok := stderr.(*object.Str); !ok || se.V != "err\n" {
		t.Fatalf("stderr = %v (%v)", stderr, err)
	}
	args, err := cp.GetAttr("args")
	mustNoErr(t, err)
	if at, ok := args.(*object.Tuple); !ok || len(at.Elts) != 1 {
		t.Fatalf("args = %v (%v)", args, err)
	}
	if _, err := cp.GetAttr("nope"); err == nil {
		t.Fatal("expected error for unknown attribute")
	}
}

func TestPopen(t *testing.T) {
	// A command that succeeds.
	p, err := NewPopen([]object.Object{object.NewStr("true")})
	if err != nil {
		t.Fatal(err)
	}
	if p.TypeName() != "popen" || p.Repr() == "" || !p.Truthy() {
		t.Fatalf("bad popen: %s %s", p.TypeName(), p.Repr())
	}
	<-p.done
	rc, err := p.GetAttr("returncode")
	if err != nil || rc.Repr() != "0" {
		t.Fatalf("returncode = %v (%v)", rc, err)
	}

	// A command that fails.
	pf, err := NewPopen([]object.Object{object.NewStr("false")})
	if err != nil {
		t.Fatal(err)
	}
	<-pf.done
	rcf, err := pf.GetAttr("returncode")
	if err != nil || rcf.Repr() != "1" {
		t.Fatalf("failing returncode = %v (%v)", rcf, err)
	}

	// Unknown attribute errors.
	if _, err := p.GetAttr("nope"); err == nil {
		t.Fatal("expected error for unknown attribute")
	}
}
