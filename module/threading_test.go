package module

import (
	"testing"
	"time"

	"gpython/internal/object"
)

type nilCaller struct{}

func (nilCaller) Call(callee object.Object, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	return nil, nil
}

func (nilCaller) Super() (object.Object, error) {
	return nil, nil
}

func callBuiltin(t *testing.T, b *object.Builtin, args ...object.Object) object.Object {
	t.Helper()
	o, err := b.Call(nilCaller{}, args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return o
}

func mustTrue(t *testing.T, o object.Object) bool {
	t.Helper()
	switch v := o.(type) {
	case object.Bool:
		return bool(v)
	case *object.Str:
		return v.V == "True"
	}
	t.Fatalf("expected bool/str, got %T (%s)", o, o.Repr())
	return false
}

func TestLockAcquireRelease(t *testing.T) {
	l := NewLock()
	acquire, err := l.GetAttr("acquire")
	if err != nil {
		t.Fatal(err)
	}
	locked, err := l.GetAttr("locked")
	if err != nil {
		t.Fatal(err)
	}
	release, err := l.GetAttr("release")
	if err != nil {
		t.Fatal(err)
	}

	// Non-blocking acquire succeeds on a free lock.
	if !mustTrue(t, callBuiltin(t, acquire.(*object.Builtin), object.BoolFrom(false))) {
		t.Fatal("expected non-blocking acquire to succeed")
	}
	if !mustTrue(t, callBuiltin(t, locked.(*object.Builtin))) {
		t.Fatal("expected lock to be held")
	}

	// Non-blocking acquire fails while held.
	if mustTrue(t, callBuiltin(t, acquire.(*object.Builtin), object.BoolFrom(false))) {
		t.Fatal("expected non-blocking acquire to fail while held")
	}

	callBuiltin(t, release.(*object.Builtin))
	if mustTrue(t, callBuiltin(t, locked.(*object.Builtin))) {
		t.Fatal("expected lock to be free after release")
	}

	// Releasing an un-acquired lock errors.
	if _, err := release.(*object.Builtin).Call(nilCaller{}, nil, nil); err == nil {
		t.Fatal("expected error releasing un-acquired lock")
	}
}

func TestRLockReentrant(t *testing.T) {
	r := NewRLock()
	acquire, err := r.GetAttr("acquire")
	if err != nil {
		t.Fatal(err)
	}
	release, err := r.GetAttr("release")
	if err != nil {
		t.Fatal(err)
	}

	// First acquire succeeds.
	if !mustTrue(t, callBuiltin(t, acquire.(*object.Builtin), object.BoolFrom(true))) {
		t.Fatal("expected first acquire to succeed")
	}
	// Reentrant acquire by the same logical caller succeeds.
	if !mustTrue(t, callBuiltin(t, acquire.(*object.Builtin), object.BoolFrom(true))) {
		t.Fatal("expected reentrant acquire to succeed")
	}
	callBuiltin(t, release.(*object.Builtin))
	callBuiltin(t, release.(*object.Builtin))
	// Releasing past count errors.
	if _, err := release.(*object.Builtin).Call(nilCaller{}, nil, nil); err == nil {
		t.Fatal("expected error releasing past count")
	}
}

func TestEventSetClearWait(t *testing.T) {
	e := NewEvent()
	isSet, err := e.GetAttr("is_set")
	if err != nil {
		t.Fatal(err)
	}
	setM, err := e.GetAttr("set")
	if err != nil {
		t.Fatal(err)
	}
	clearM, err := e.GetAttr("clear")
	if err != nil {
		t.Fatal(err)
	}
	waitM, err := e.GetAttr("wait")
	if err != nil {
		t.Fatal(err)
	}

	if mustTrue(t, callBuiltin(t, isSet.(*object.Builtin))) {
		t.Fatal("expected event to be clear initially")
	}
	callBuiltin(t, setM.(*object.Builtin))
	if !mustTrue(t, callBuiltin(t, isSet.(*object.Builtin))) {
		t.Fatal("expected event to be set")
	}
	// wait with timeout 0 returns immediately (already set -> True).
	if !mustTrue(t, callBuiltin(t, waitM.(*object.Builtin), object.NewInt(0))) {
		t.Fatal("expected wait(0) to return True when set")
	}
	callBuiltin(t, clearM.(*object.Builtin))
	if mustTrue(t, callBuiltin(t, isSet.(*object.Builtin))) {
		t.Fatal("expected event to be clear")
	}
	// wait with timeout 0 returns False when clear.
	if mustTrue(t, callBuiltin(t, waitM.(*object.Builtin), object.NewInt(0))) {
		t.Fatal("expected wait(0) to return False when clear")
	}

	// Background set then wait should return True.
	done := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		callBuiltin(t, setM.(*object.Builtin))
		close(done)
	}()
	if !mustTrue(t, callBuiltin(t, waitM.(*object.Builtin), object.NewInt(1))) {
		t.Fatal("expected wait(1) to return True after set")
	}
	<-done
}
