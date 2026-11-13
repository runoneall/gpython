package object

import (
	"errors"
	"fmt"
)

// ErrIterationDone is returned by iterators when they are exhausted.
// It is deliberately not a Python exception: the for-statement
// translates it into a normal loop exit, while next() converts it to
// StopIteration.
var ErrIterationDone = errors.New("iteration done")

// Iterator is the low-level iteration protocol used internally.
type Iterator interface {
	Next() (Object, error)
}

type genMsg struct {
	value Object
	err   error
	done  bool
}

// Generator is a lazily executed generator. The body runs in a
// dedicated goroutine which suspends at each yield, matching Python's
// resumable execution model.
type Generator struct {
	name     string
	run      func(*Generator) error
	reqs     chan struct{}
	outs     chan genMsg
	started  bool
	finished bool
}

// NewGenerator creates a generator whose body is run.
func NewGenerator(name string, run func(*Generator) error) *Generator {
	return &Generator{name: name, run: run, reqs: make(chan struct{}), outs: make(chan genMsg)}
}

func (*Generator) TypeName() string { return "generator" }
func (g *Generator) Repr() string   { return "<generator object " + g.name + ">" }
func (g *Generator) Str() string    { return g.Repr() }
func (*Generator) Truthy() bool     { return true }

// Next resumes the generator and returns the next yielded value.
func (g *Generator) Next() (Object, error) {
	if g.finished {
		return nil, ErrIterationDone
	}
	if !g.started {
		g.started = true
		go func() {
			defer func() {
				if r := recover(); r != nil {
					g.outs <- genMsg{err: fmt.Errorf("internal generator error")}
				}
			}()
			if err := g.run(g); err != nil {
				g.outs <- genMsg{err: err}
				return
			}
			g.outs <- genMsg{done: true}
		}()
	} else {
		g.reqs <- struct{}{}
	}
	msg := <-g.outs
	if msg.done {
		g.finished = true
		return nil, ErrIterationDone
	}
	if msg.err != nil {
		g.finished = true
		return nil, msg.err
	}
	return msg.value, nil
}

// Yield publishes a value and blocks until the consumer asks for the
// next one. Called from the generator's goroutine.
func (g *Generator) Yield(v Object) {
	g.outs <- genMsg{value: v}
	<-g.reqs
}

// Close marks the generator finished. Used by the evaluator when a
// generator body returns.
func (g *Generator) Close() { g.finished = true }

func generatorAttr(g *Generator, name string) (Object, error) {
	switch name {
	case "__iter__":
		return NewBuiltin("__iter__", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return g, nil
		}), nil
	case "__next__":
		return NewBuiltin("__next__", func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			v, err := g.Next()
			if err == ErrIterationDone {
				return nil, Raise(StopIteration)
			}
			return v, err
		}), nil
	case "send", "throw", "close":
		return NewBuiltin(name, func(ctx Caller, args []Object, kwargs map[string]Object) (Object, error) {
			return nil, Raisef(NotImplementedError, "generator.%s() is not implemented", name)
		}), nil
	}
	return nil, attrError("generator", name)
}

// IterObject adapts a low-level Iterator to a Python iterator object.
type IterObject struct {
	It Iterator
}

func NewIterObject(it Iterator) *IterObject { return &IterObject{It: it} }

func (*IterObject) TypeName() string { return "iterator" }
func (o *IterObject) Repr() string   { return "<iterator object>" }
func (o *IterObject) Str() string    { return o.Repr() }
func (*IterObject) Truthy() bool     { return true }
func (o *IterObject) Next() (Object, error) {
	return o.It.Next()
}
