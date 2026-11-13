package interp

import "gpython/internal/object"

// EnvKind distinguishes the three Python namespace flavours.
type EnvKind int

const (
	EnvModule EnvKind = iota
	EnvFunction
	EnvClass
)

// Env is a lexical scope. A function environment skips enclosing class
// namespaces when resolving free variables, matching Python's LEGB
// rules.
type Env struct {
	Vars     map[string]object.Object
	Parent   *Env
	Globals  *Env
	Kind     EnvKind
	Name     string
	Global   map[string]bool
	Nonlocal map[string]bool
}

func newEnv(kind EnvKind, name string, parent *Env) *Env {
	e := &Env{
		Vars:     map[string]object.Object{},
		Parent:   parent,
		Kind:     kind,
		Name:     name,
		Global:   map[string]bool{},
		Nonlocal: map[string]bool{},
	}
	if parent != nil {
		e.Globals = parent.Globals
	}
	return e
}

// NewModuleEnv creates the top-level namespace of a module.
func NewModuleEnv(name string) *Env {
	e := newEnv(EnvModule, name, nil)
	e.Globals = e
	return e
}

// Child creates a nested scope.
func (e *Env) Child(kind EnvKind, name string) *Env {
	return newEnv(kind, name, e)
}

func (e *Env) declareGlobal(name string) {
	e.Global[name] = true
	delete(e.Nonlocal, name)
}

func (e *Env) declareNonlocal(name string) {
	e.Nonlocal[name] = true
	delete(e.Global, name)
}

// Get resolves a name against the lexical chain. Builtins are handled
// by the interpreter after this returns false.
func (e *Env) Get(name string) (object.Object, bool) {
	if e.Kind == EnvFunction {
		if e.Global[name] {
			v, ok := e.Globals.Vars[name]
			return v, ok
		}
		if e.Nonlocal[name] {
			for cur := e.Parent; cur != nil; cur = cur.Parent {
				if cur.Kind == EnvClass {
					continue
				}
				if cur.Kind == EnvModule {
					break
				}
				if v, ok := cur.Vars[name]; ok {
					return v, true
				}
			}
			return nil, false
		}
	}
	first := true
	for cur := e; cur != nil; cur = cur.Parent {
		if cur.Kind == EnvClass && !first {
			first = false
			continue
		}
		if v, ok := cur.Vars[name]; ok {
			return v, true
		}
		first = false
	}
	return nil, false
}

// Set assigns a name, honouring global/nonlocal declarations.
func (e *Env) Set(name string, v object.Object) {
	if e.Kind == EnvFunction {
		if e.Global[name] {
			e.Globals.Vars[name] = v
			return
		}
		if e.Nonlocal[name] {
			for cur := e.Parent; cur != nil; cur = cur.Parent {
				if cur.Kind == EnvClass {
					continue
				}
				if cur.Kind == EnvModule {
					break
				}
				if _, ok := cur.Vars[name]; ok {
					cur.Vars[name] = v
					return
				}
			}
		}
	}
	e.Vars[name] = v
}

// Delete removes a local (or global/nonlocal) binding.
func (e *Env) Delete(name string) bool {
	if e.Kind == EnvFunction {
		if e.Global[name] {
			if _, ok := e.Globals.Vars[name]; ok {
				delete(e.Globals.Vars, name)
				return true
			}
			return false
		}
		if e.Nonlocal[name] {
			for cur := e.Parent; cur != nil; cur = cur.Parent {
				if cur.Kind == EnvClass {
					continue
				}
				if _, ok := cur.Vars[name]; ok {
					delete(cur.Vars, name)
					return true
				}
			}
			return false
		}
	}
	if _, ok := e.Vars[name]; ok {
		delete(e.Vars, name)
		return true
	}
	return false
}
