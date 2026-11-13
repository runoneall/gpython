package module

import (
	"sync"
	"sync/atomic"
	"time"

	"gpython/internal/object"
)

// Thread is a Python threading.Thread-compatible value. The target
// callable runs in a dedicated Go goroutine started by start().
type Thread struct {
	target object.Object
	args   []object.Object
	kwargs map[string]object.Object
	name   string
	daemon bool

	mu       sync.Mutex
	started  bool
	finished bool
	done     chan struct{}
	err      error
}

// NewThread builds a Thread value (matching threading.Thread(target, args, kwargs, name, daemon)).
func NewThread(target object.Object, args []object.Object, kwargs map[string]object.Object, name string, daemon bool) *Thread {
	return &Thread{
		target: target,
		args:   args,
		kwargs: kwargs,
		name:   name,
		daemon: daemon,
		done:   make(chan struct{}),
	}
}

func (t *Thread) TypeName() string { return "thread" }
func (t *Thread) Repr() string {
	state := "initial"
	if t.started {
		state = "started"
	}
	return "<Thread(" + t.name + ", " + state + ")>"
}
func (t *Thread) Str() string { return t.Repr() }
func (*Thread) Truthy() bool  { return true }

// GetAttr exposes Thread methods: start, join, is_alive, plus name/daemon.
func (t *Thread) GetAttr(name string) (object.Object, error) {
	switch name {
	case "name":
		return object.NewStr(t.name), nil
	case "daemon":
		return object.BoolFrom(t.daemon), nil
	case "is_alive":
		return object.NewBuiltin("is_alive", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "is_alive() takes no arguments")
			}
			t.mu.Lock()
			alive := t.started && !t.finished
			t.mu.Unlock()
			return object.BoolFrom(alive), nil
		}), nil
	case "start":
		return object.NewBuiltin("start", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "start() takes no arguments")
			}
			t.mu.Lock()
			if t.started {
				t.mu.Unlock()
				return nil, object.Raisef(object.RuntimeError, "threads can only be started once")
			}
			t.started = true
			t.mu.Unlock()

			go func() {
				_, err := ctx.Call(t.target, t.args, t.kwargs)
				t.mu.Lock()
				t.finished = true
				t.err = err
				t.mu.Unlock()
				close(t.done)
			}()
			return object.None, nil
		}), nil
	case "join":
		return object.NewBuiltin("join", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "join() takes no arguments")
			}
			<-t.done
			t.mu.Lock()
			err := t.err
			t.mu.Unlock()
			if err != nil {
				return nil, err
			}
			return object.None, nil
		}), nil
	}
	return nil, object.Raisef(object.AttributeError, "'Thread' object has no attribute '%s'", name)
}

func (t *Thread) SetAttr(name string, value object.Object) error {
	return object.Raisef(object.AttributeError, "'Thread' object attribute '%s' is read-only", name)
}

// Lock is a primitive lock (threading.Lock). Uses atomic state to avoid
// self-deadlock when querying lock status.
type Lock struct {
	held atomic.Int32 // 0 = free, 1 = held
}

func NewLock() *Lock { return &Lock{} }

func (*Lock) TypeName() string { return "lock" }
func (l *Lock) Repr() string   { return "<unlocked lock>" }
func (l *Lock) Str() string    { return l.Repr() }
func (*Lock) Truthy() bool     { return true }

func (l *Lock) GetAttr(name string) (object.Object, error) {
	switch name {
	case "acquire":
		return object.NewBuiltin("acquire", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			blocking := true
			timeout := -1.0
			_ = timeout
			for k, v := range kwargs {
				switch k {
				case "blocking":
					b, ok := v.(object.Bool)
					if !ok {
						return nil, object.Raisef(object.TypeErrorClass, "blocking must be bool")
					}
					blocking = bool(b)
				case "timeout":
					f, ok := toFloat(v)
					if !ok {
						return nil, object.Raisef(object.TypeErrorClass, "timeout must be a number")
					}
					timeout = f
				}
			}
			if len(args) > 1 {
				return nil, object.Raisef(object.TypeErrorClass, "acquire() takes at most 1 positional argument")
			}
			if len(args) == 1 {
				b, ok := args[0].(object.Bool)
				if !ok {
					return nil, object.Raisef(object.TypeErrorClass, "blocking must be bool")
				}
				blocking = bool(b)
			}
			if !blocking {
				return object.BoolFrom(l.held.CompareAndSwap(0, 1)), nil
			}
			if timeout < 0 {
				for !l.held.CompareAndSwap(0, 1) {
					time.Sleep(time.Millisecond)
				}
				return object.True, nil
			}
			timer := time.NewTimer(time.Duration(timeout * float64(time.Second)))
			defer timer.Stop()
			for {
				if l.held.CompareAndSwap(0, 1) {
					return object.True, nil
				}
				select {
				case <-timer.C:
					return object.False, nil
				default:
					time.Sleep(time.Millisecond)
				}
			}
		}), nil
	case "release":
		return object.NewBuiltin("release", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "release() takes no arguments")
			}
			if !l.held.CompareAndSwap(1, 0) {
				return nil, object.Raisef(object.RuntimeError, "cannot release un-acquired lock")
			}
			return object.None, nil
		}), nil
	case "locked":
		return object.NewBuiltin("locked", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "locked() takes no arguments")
			}
			return object.BoolFrom(l.held.Load() == 1), nil
		}), nil
	}
	return nil, object.Raisef(object.AttributeError, "'Lock' object has no attribute '%s'", name)
}

func (l *Lock) SetAttr(name string, value object.Object) error {
	return object.Raisef(object.AttributeError, "'Lock' object attribute '%s' is read-only", name)
}

// RLock is a reentrant lock (threading.RLock).
type RLock struct {
	mu    sync.Mutex
	count int32
	held  bool
}

func NewRLock() *RLock { return &RLock{} }

func (*RLock) TypeName() string { return "rlock" }
func (r *RLock) Repr() string   { return "<unlocked RLock>" }
func (r *RLock) Str() string    { return r.Repr() }
func (*RLock) Truthy() bool     { return true }

func (r *RLock) GetAttr(name string) (object.Object, error) {
	switch name {
	case "acquire":
		return object.NewBuiltin("acquire", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			blocking := true
			timeout := -1.0
			_ = timeout
			for k, v := range kwargs {
				switch k {
				case "blocking":
					b, ok := v.(object.Bool)
					if !ok {
						return nil, object.Raisef(object.TypeErrorClass, "blocking must be bool")
					}
					blocking = bool(b)
				case "timeout":
					f, ok := toFloat(v)
					if !ok {
						return nil, object.Raisef(object.TypeErrorClass, "timeout must be a number")
					}
					timeout = f
				}
			}
			if len(args) > 1 {
				return nil, object.Raisef(object.TypeErrorClass, "acquire() takes at most 1 positional argument")
			}
			if len(args) == 1 {
				b, ok := args[0].(object.Bool)
				if !ok {
					return nil, object.Raisef(object.TypeErrorClass, "blocking must be bool")
				}
				blocking = bool(b)
			}
			if !blocking {
				r.mu.Lock()
				got := !r.held
				if got {
					r.held = true
					r.count = 1
				}
				r.mu.Unlock()
				return object.BoolFrom(got), nil
			}
			r.mu.Lock()
			if r.held {
				// Reentrant: same logical caller already holds the lock.
				r.count++
				r.mu.Unlock()
				return object.True, nil
			}
			r.held = true
			r.count = 1
			r.mu.Unlock()
			_ = timeout
			return object.True, nil
		}), nil
	case "release":
		return object.NewBuiltin("release", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "release() takes no arguments")
			}
			r.mu.Lock()
			if r.count == 0 {
				r.mu.Unlock()
				return nil, object.Raisef(object.RuntimeError, "cannot release un-acquired lock")
			}
			r.count--
			if r.count == 0 {
				r.held = false
			}
			r.mu.Unlock()
			return object.None, nil
		}), nil
	}
	return nil, object.Raisef(object.AttributeError, "'RLock' object has no attribute '%s'", name)
}

func (r *RLock) SetAttr(name string, value object.Object) error {
	return object.Raisef(object.AttributeError, "'RLock' object attribute '%s' is read-only", name)
}

// Event is a thread event (threading.Event).
type Event struct {
	mu   sync.Mutex
	cond *sync.Cond
	set  bool
}

func NewEvent() *Event {
	e := &Event{}
	e.cond = sync.NewCond(&e.mu)
	return e
}

func (*Event) TypeName() string { return "event" }
func (e *Event) Repr() string   { return "<Event object>" }
func (e *Event) Str() string    { return e.Repr() }
func (*Event) Truthy() bool     { return true }

func (e *Event) GetAttr(name string) (object.Object, error) {
	switch name {
	case "is_set":
		return object.NewBuiltin("is_set", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "is_set() takes no arguments")
			}
			e.mu.Lock()
			s := e.set
			e.mu.Unlock()
			return object.BoolFrom(s), nil
		}), nil
	case "set":
		return object.NewBuiltin("set", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "set() takes no arguments")
			}
			e.mu.Lock()
			e.set = true
			e.cond.Broadcast()
			e.mu.Unlock()
			return object.None, nil
		}), nil
	case "clear":
		return object.NewBuiltin("clear", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "clear() takes no arguments")
			}
			e.mu.Lock()
			e.set = false
			e.mu.Unlock()
			return object.None, nil
		}), nil
	case "wait":
		return object.NewBuiltin("wait", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			timeout := -1.0
			if len(args) > 1 {
				return nil, object.Raisef(object.TypeErrorClass, "wait() takes at most 1 argument")
			}
			if len(args) == 1 {
				f, ok := toFloat(args[0])
				if !ok {
					return nil, object.Raisef(object.TypeErrorClass, "timeout must be a number")
				}
				timeout = f
			}
			if v, ok := kwargs["timeout"]; ok {
				f, ok := toFloat(v)
				if !ok {
					return nil, object.Raisef(object.TypeErrorClass, "timeout must be a number")
				}
				timeout = f
			}
			delete(kwargs, "timeout")
			if len(kwargs) > 0 {
				for k := range kwargs {
					return nil, object.Raisef(object.TypeErrorClass, "wait() got an unexpected keyword argument '%s'", k)
				}
			}
			e.mu.Lock()
			if e.set {
				e.mu.Unlock()
				return object.True, nil
			}
			if timeout < 0 {
				for !e.set {
					e.cond.Wait()
				}
				e.mu.Unlock()
				return object.BoolFrom(e.set), nil
			}
			deadline := time.Now().Add(time.Duration(timeout * float64(time.Second)))
			for !e.set {
				if time.Now().After(deadline) {
					e.mu.Unlock()
					return object.False, nil
				}
				e.mu.Unlock()
				time.Sleep(time.Millisecond)
				e.mu.Lock()
			}
			e.mu.Unlock()
			return object.BoolFrom(e.set), nil
		}), nil
	}
	return nil, object.Raisef(object.AttributeError, "'Event' object has no attribute '%s'", name)
}

func (e *Event) SetAttr(name string, value object.Object) error {
	return object.Raisef(object.AttributeError, "'Event' object attribute '%s' is read-only", name)
}

func newThreadingModule() *object.Module {
	m := object.NewModule("threading")

	m.Dict["Thread"] = object.NewBuiltin("Thread", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if kwargs == nil {
			kwargs = map[string]object.Object{}
		}
		var target object.Object
		var argsObj []object.Object
		name := "Thread-" + nextThreadName()
		daemon := false

		// Positional: target, args, kwargs
		if len(args) > 0 {
			target = args[0]
		}
		if len(args) > 1 {
			switch a := args[1].(type) {
			case *object.Tuple:
				argsObj = a.Elts
			case *object.List:
				argsObj = a.Elts
			default:
				return nil, object.Raisef(object.TypeErrorClass, "args must be a tuple or list")
			}
		}
		if len(args) > 2 {
			switch kw := args[2].(type) {
			case *object.Dict:
				for i, k := range kw.Keys {
					s, ok := k.(*object.Str)
					if !ok {
						return nil, object.Raisef(object.TypeErrorClass, "kwargs keys must be str")
					}
					kwargs[s.V] = kw.Values[i]
				}
			default:
				return nil, object.Raisef(object.TypeErrorClass, "kwargs must be a dict")
			}
		}
		for k, v := range kwargs {
			switch k {
			case "target":
				if target != nil {
					return nil, object.Raisef(object.TypeErrorClass, "got multiple values for argument 'target'")
				}
				target = v
				delete(kwargs, k)
			case "name":
				if s, ok := v.(*object.Str); ok {
					name = s.V
				} else {
					return nil, object.Raisef(object.TypeErrorClass, "name must be str")
				}
				delete(kwargs, k)
			case "daemon":
				if b, ok := v.(object.Bool); ok {
					daemon = bool(b)
				} else {
					return nil, object.Raisef(object.TypeErrorClass, "daemon must be bool")
				}
				delete(kwargs, k)
			case "args":
				if t, ok := v.(*object.Tuple); ok {
					argsObj = t.Elts
				} else if t, ok := v.(*object.List); ok {
					argsObj = t.Elts
				} else {
					return nil, object.Raisef(object.TypeErrorClass, "args must be a tuple or list")
				}
				delete(kwargs, k)
			case "kwargs":
				if d, ok := v.(*object.Dict); ok {
					for i, k := range d.Keys {
						s, ok := k.(*object.Str)
						if !ok {
							return nil, object.Raisef(object.TypeErrorClass, "kwargs keys must be str")
						}
						kwargs[s.V] = d.Values[i]
					}
				} else {
					return nil, object.Raisef(object.TypeErrorClass, "kwargs must be a dict")
				}
				delete(kwargs, k)
			}
		}
		if target == nil {
			return nil, object.Raisef(object.TypeErrorClass, "Thread() missing required argument: 'target'")
		}
		return NewThread(target, argsObj, kwargs, name, daemon), nil
	})

	m.Dict["Lock"] = object.NewBuiltin("Lock", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "Lock() takes no arguments")
		}
		return NewLock(), nil
	})
	m.Dict["RLock"] = object.NewBuiltin("RLock", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "RLock() takes no arguments")
		}
		return NewRLock(), nil
	})
	m.Dict["Event"] = object.NewBuiltin("Event", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "Event() takes no arguments")
		}
		return NewEvent(), nil
	})

	m.Dict["active_count"] = object.NewBuiltin("active_count", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "active_count() takes no arguments")
		}
		return object.NewInt(int64(activeThreadCount())), nil
	})
	m.Dict["current_thread"] = object.NewBuiltin("current_thread", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "current_thread() takes no arguments")
		}
		return NewThread(nil, nil, nil, "MainThread", false), nil
	})

	return m
}

func init() {
	Register("threading", newThreadingModule)
}

var threadNameCounter int64

func nextThreadName() string {
	return string(rune('0' + atomic.AddInt64(&threadNameCounter, 1)%10))
}

func activeThreadCount() int {
	// gpython tracks only the main thread explicitly; worker threads are not registered.
	return 1
}

// toFloat converts any numeric object to float64.
func toFloat(v object.Object) (float64, bool) {
	switch x := v.(type) {
	case *object.Int:
		return float64(x.V), true
	case *object.Float:
		return x.V, true
	case object.Bool:
		if bool(x) {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
