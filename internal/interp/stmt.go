package interp

import (
	"gpython/internal/object"

	"github.com/tamnd/gopapy/v2/parser2"
)

func (i *Interpreter) evalStmt(s parser2.Stmt, env *Env) error {
	switch n := s.(type) {
	case *parser2.ExprStmt:
		_, err := i.evalExpr(n.Value, env)
		return err
	case *parser2.Assign:
		v, err := i.evalExpr(n.Value, env)
		if err != nil {
			return err
		}
		for _, target := range n.Targets {
			if err := i.assignTarget(target, v, env); err != nil {
				return err
			}
		}
		return nil
	case *parser2.AugAssign:
		return i.evalAugAssign(n, env)
	case *parser2.AnnAssign:
		if n.Value == nil {
			return nil
		}
		v, err := i.evalExpr(n.Value, env)
		if err != nil {
			return err
		}
		return i.assignTarget(n.Target, v, env)
	case *parser2.Return:
		if i.Curr == nil || !i.Curr.IsFunction {
			return syntaxAt(i.currentFile(), n.P, "'return' outside function")
		}
		var v object.Object = object.None
		if n.Value != nil {
			var err error
			v, err = i.evalExpr(n.Value, env)
			if err != nil {
				return err
			}
		}
		return &returnSignal{value: v, pos: n.P}
	case *parser2.Raise:
		return i.evalRaise(n, env)
	case *parser2.Pass:
		return nil
	case *parser2.Break:
		if i.loopDepth <= 0 {
			return syntaxAt(i.currentFile(), n.P, "'break' outside loop")
		}
		return &breakSignal{pos: n.P}
	case *parser2.Continue:
		if i.loopDepth <= 0 {
			return syntaxAt(i.currentFile(), n.P, "'continue' outside loop")
		}
		return &continueSignal{pos: n.P}
	case *parser2.Import:
		return i.evalImport(n, env)
	case *parser2.ImportFrom:
		return i.evalImportFrom(n, env)
	case *parser2.Global:
		for _, name := range n.Names {
			env.declareGlobal(name)
		}
		return nil
	case *parser2.Nonlocal:
		for _, name := range n.Names {
			env.declareNonlocal(name)
		}
		return nil
	case *parser2.Delete:
		for _, target := range n.Targets {
			if err := i.deleteTarget(target, env); err != nil {
				return err
			}
		}
		return nil
	case *parser2.Assert:
		test, err := i.evalExpr(n.Test, env)
		if err != nil {
			return err
		}
		if test.Truthy() {
			return nil
		}
		msg := object.Object(object.NewStr(""))
		if n.Msg != nil {
			msg, err = i.evalExpr(n.Msg, env)
			if err != nil {
				return err
			}
		}
		return object.Raise(object.AssertionError, msg)
	case *parser2.If:
		test, err := i.evalExpr(n.Test, env)
		if err != nil {
			return err
		}
		if test.Truthy() {
			return i.EvalStmts(n.Body, env)
		}
		return i.EvalStmts(n.Orelse, env)
	case *parser2.While:
		return i.evalWhile(n, env)
	case *parser2.For:
		return i.evalFor(n, env)
	case *parser2.Try:
		return i.evalTry(n, env)
	case *parser2.FunctionDef:
		return i.evalFunctionDef(n, env)
	case *parser2.ClassDef:
		return i.evalClassDef(n, env)
	case *parser2.AsyncFunctionDef, *parser2.AsyncFor, *parser2.AsyncWith:
		return &FatalError{File: i.currentFile(), Line: nodePos(s).Line, Col: nodePos(s).Col, Msg: "notimplementederror: async/await is not supported"}
	case *parser2.With:
		return unimplemented(nodePos(s), "with statement")
	case *parser2.Match:
		return unimplemented(nodePos(s), "match statement")
	case *parser2.TypeAlias:
		return unimplemented(nodePos(s), "type alias")
	}
	return unimplemented(nodePos(s), "statement")
}

func (i *Interpreter) evalAugAssign(n *parser2.AugAssign, env *Env) error {
	switch target := n.Target.(type) {
	case *parser2.Name:
		old, err := i.lookupName(env, target.Id)
		if err != nil {
			return err
		}
		val, err := i.evalExpr(n.Value, env)
		if err != nil {
			return err
		}
		res, err := object.BinaryOp(n.Op, old, val)
		if err != nil {
			return err
		}
		env.Set(target.Id, res)
		return nil
	case *parser2.Attribute:
		obj, err := i.evalExpr(target.Value, env)
		if err != nil {
			return err
		}
		old, err := object.GetAttr(obj, target.Attr)
		if err != nil {
			return err
		}
		val, err := i.evalExpr(n.Value, env)
		if err != nil {
			return err
		}
		res, err := object.BinaryOp(n.Op, old, val)
		if err != nil {
			return err
		}
		return object.SetAttr(obj, target.Attr, res)
	case *parser2.Subscript:
		obj, err := i.evalExpr(target.Value, env)
		if err != nil {
			return err
		}
		key, err := i.evalSubscriptKey(target.Slice, env)
		if err != nil {
			return err
		}
		old, err := object.GetItem(obj, key)
		if err != nil {
			return err
		}
		val, err := i.evalExpr(n.Value, env)
		if err != nil {
			return err
		}
		res, err := object.BinaryOp(n.Op, old, val)
		if err != nil {
			return err
		}
		return object.SetItem(obj, key, res)
	}
	return object.Raisef(object.SyntaxErrorClass, "illegal target for augmented assignment")
}

func (i *Interpreter) evalRaise(n *parser2.Raise, env *Env) error {
	var value object.Object
	if n.Exc == nil {
		if len(i.excStack) == 0 {
			return object.Raisef(object.RuntimeError, "no active exception to reraise")
		}
		value = i.excStack[len(i.excStack)-1]
	} else {
		v, err := i.evalExpr(n.Exc, env)
		if err != nil {
			return err
		}
		value = v
	}
	inst, err := i.exceptionValue(value)
	if err != nil {
		return err
	}
	if n.Cause != nil {
		cause, err := i.evalExpr(n.Cause, env)
		if err != nil {
			return err
		}
		if cause != object.None {
			inst.Dict["__cause__"] = cause
		}
	}
	return &object.PyError{Value: inst}
}

func (i *Interpreter) exceptionValue(v object.Object) (*object.Instance, error) {
	switch x := v.(type) {
	case *object.Instance:
		if !x.Class.IsException {
			return nil, object.Raisef(object.TypeErrorClass, "exceptions must derive from baseexception")
		}
		return x, nil
	case *object.Class:
		if !x.IsException {
			return nil, object.Raisef(object.TypeErrorClass, "exceptions must derive from baseexception")
		}
		return object.NewException(x), nil
	}
	return nil, object.Raisef(object.TypeErrorClass, "exceptions must derive from baseexception")
}

func (i *Interpreter) evalWhile(n *parser2.While, env *Env) error {
	i.loopDepth++
	defer func() { i.loopDepth-- }()
	broke := false
	for {
		test, err := i.evalExpr(n.Test, env)
		if err != nil {
			return err
		}
		if !test.Truthy() {
			break
		}
		err = i.EvalStmts(n.Body, env)
		if err != nil {
			if _, ok := err.(*breakSignal); ok {
				broke = true
				break
			}
			if _, ok := err.(*continueSignal); ok {
				continue
			}
			return err
		}
	}
	if !broke {
		return i.EvalStmts(n.Orelse, env)
	}
	return nil
}

func (i *Interpreter) evalFor(n *parser2.For, env *Env) error {
	iv, err := i.evalExpr(n.Iter, env)
	if err != nil {
		return err
	}
	it, err := object.Iterate(iv)
	if err != nil {
		return err
	}
	i.loopDepth++
	defer func() { i.loopDepth-- }()
	broke := false
	for {
		v, err := it.Next()
		if err == object.ErrIterationDone {
			break
		}
		if err != nil {
			return err
		}
		if err := i.assignTarget(n.Target, v, env); err != nil {
			return err
		}
		err = i.EvalStmts(n.Body, env)
		if err != nil {
			if _, ok := err.(*breakSignal); ok {
				broke = true
				break
			}
			if _, ok := err.(*continueSignal); ok {
				continue
			}
			return err
		}
	}
	if !broke {
		return i.EvalStmts(n.Orelse, env)
	}
	return nil
}

func (i *Interpreter) evalTry(n *parser2.Try, env *Env) error {
	err := i.EvalStmts(n.Body, env)
	if err == nil && len(n.Orelse) > 0 {
		err = i.EvalStmts(n.Orelse, env)
	} else if pe, ok := err.(*object.PyError); ok {
		handled := false
		for _, h := range n.Handlers {
			match, merr := i.handlerMatches(h, pe.Value, env)
			if merr != nil {
				err = merr
				handled = true
				break
			}
			if !match {
				continue
			}
			if h.Name != "" {
				env.Set(h.Name, pe.Value)
			}
			i.excStack = append(i.excStack, pe.Value)
			err = i.EvalStmts(h.Body, env)
			i.excStack = i.excStack[:len(i.excStack)-1]
			if h.Name != "" {
				env.Delete(h.Name)
			}
			handled = true
			break
		}
		if !handled {
			err = pe
		}
	}
	if len(n.Finalbody) > 0 {
		if ferr := i.EvalStmts(n.Finalbody, env); ferr != nil {
			return ferr
		}
	}
	return err
}

func (i *Interpreter) handlerMatches(h *parser2.ExceptHandler, exc object.Object, env *Env) (bool, error) {
	if h.Type == nil {
		return true, nil
	}
	tv, err := i.evalExpr(h.Type, env)
	if err != nil {
		return false, err
	}
	if cls, ok := tv.(*object.Class); ok && cls.IsException {
		return object.IsInstance(exc, cls)
	}
	if tup, ok := tv.(*object.Tuple); ok {
		for _, e := range tup.Elts {
			c, ok := e.(*object.Class)
			if !ok || !c.IsException {
				return false, object.Raisef(object.TypeErrorClass, "catching classes that do not inherit from baseexception is not allowed")
			}
		}
		return object.IsInstance(exc, tup)
	}
	return false, object.Raisef(object.TypeErrorClass, "catching classes that do not inherit from baseexception is not allowed")
}

func (i *Interpreter) evalFunctionDef(n *parser2.FunctionDef, env *Env) error {
	defaults, kwDefaults, err := i.evalDefaults(n.Args, env)
	if err != nil {
		return err
	}
	decorators := make([]object.Object, len(n.DecoratorList))
	for idx, d := range n.DecoratorList {
		decorators[idx], err = i.evalExpr(d, env)
		if err != nil {
			return err
		}
	}
	var fn object.Object = &object.Function{
		Name:        n.Name,
		Args:        n.Args,
		Defaults:    defaults,
		KwDefaults:  kwDefaults,
		Body:        n.Body,
		Closure:     env,
		File:        i.currentFile(),
		IsGenerator: hasYield(n.Body),
	}
	for idx := len(decorators) - 1; idx >= 0; idx-- {
		fn, err = i.Call(decorators[idx], []object.Object{fn}, nil)
		if err != nil {
			return err
		}
	}
	env.Set(n.Name, fn)
	return nil
}

func (i *Interpreter) evalClassDef(n *parser2.ClassDef, env *Env) error {
	bases := make([]*object.Class, 0, len(n.Bases))
	for _, be := range n.Bases {
		bv, err := i.evalExpr(be, env)
		if err != nil {
			return err
		}
		cls, ok := bv.(*object.Class)
		if !ok {
			return object.Raisef(object.TypeErrorClass, "class bases must be classes")
		}
		bases = append(bases, cls)
	}
	for _, kw := range n.Keywords {
		if kw.Arg == "metaclass" {
			return object.Raisef(object.NotImplementedError, "metaclasses are not supported")
		}
		return object.Raisef(object.TypeErrorClass, "unexpected class keyword '%s'", kw.Arg)
	}
	decorators := make([]object.Object, len(n.DecoratorList))
	for idx, d := range n.DecoratorList {
		v, err := i.evalExpr(d, env)
		if err != nil {
			return err
		}
		decorators[idx] = v
	}

	classEnv := env.Child(EnvClass, n.Name)
	classEnv.Vars["__name__"] = object.NewStr(n.Name)
	if err := i.EvalStmts(n.Body, classEnv); err != nil {
		return err
	}

	class := object.NewClass(n.Name, bases, "")
	for k, v := range classEnv.Vars {
		class.Dict[k] = v
	}
	for _, v := range class.Dict {
		if fn, ok := v.(*object.Function); ok {
			fn.DefClass = class
		}
	}

	var obj object.Object = class
	var err error
	for idx := len(decorators) - 1; idx >= 0; idx-- {
		obj, err = i.Call(decorators[idx], []object.Object{obj}, nil)
		if err != nil {
			return err
		}
	}
	env.Set(n.Name, obj)
	return nil
}

func (i *Interpreter) assignTarget(target parser2.Expr, value object.Object, env *Env) error {
	switch t := target.(type) {
	case *parser2.Name:
		env.Set(t.Id, value)
		return nil
	case *parser2.Attribute:
		obj, err := i.evalExpr(t.Value, env)
		if err != nil {
			return err
		}
		return object.SetAttr(obj, t.Attr, value)
	case *parser2.Subscript:
		obj, err := i.evalExpr(t.Value, env)
		if err != nil {
			return err
		}
		key, err := i.evalSubscriptKey(t.Slice, env)
		if err != nil {
			return err
		}
		return object.SetItem(obj, key, value)
	case *parser2.Tuple:
		return i.assignSequence(t.Elts, value, env)
	case *parser2.List:
		return i.assignSequence(t.Elts, value, env)
	case *parser2.Starred:
		it, err := object.Iterate(value)
		if err != nil {
			return err
		}
		var out []object.Object
		for {
			v, err := it.Next()
			if err == object.ErrIterationDone {
				break
			}
			if err != nil {
				return err
			}
			out = append(out, v)
		}
		return i.assignTarget(t.Value, object.NewList(out...), env)
	}
	return object.Raisef(object.SyntaxErrorClass, "cannot assign to this expression")
}

func (i *Interpreter) assignSequence(targets []parser2.Expr, value object.Object, env *Env) error {
	vals, err := sequenceValues(value)
	if err != nil {
		return err
	}
	star := -1
	for idx, t := range targets {
		if _, ok := t.(*parser2.Starred); ok {
			star = idx
			break
		}
	}
	if star < 0 {
		if len(vals) != len(targets) {
			if len(vals) < len(targets) {
				return object.Raisef(object.ValueErrorClass, "not enough values to unpack (expected %d, got %d)", len(targets), len(vals))
			}
			return object.Raisef(object.ValueErrorClass, "too many values to unpack (expected %d)", len(targets))
		}
		for idx, t := range targets {
			if err := i.assignTarget(t, vals[idx], env); err != nil {
				return err
			}
		}
		return nil
	}
	trailing := len(targets) - star - 1
	if len(vals) < len(targets)-1 {
		return object.Raisef(object.ValueErrorClass, "not enough values to unpack (expected at least %d, got %d)", len(targets)-1, len(vals))
	}
	for idx := 0; idx < star; idx++ {
		if err := i.assignTarget(targets[idx], vals[idx], env); err != nil {
			return err
		}
	}
	starTarget := targets[star].(*parser2.Starred)
	mid := vals[star : len(vals)-trailing]
	if err := i.assignTarget(starTarget.Value, object.NewList(mid...), env); err != nil {
		return err
	}
	for idx := 0; idx < trailing; idx++ {
		if err := i.assignTarget(targets[star+1+idx], vals[len(vals)-trailing+idx], env); err != nil {
			return err
		}
	}
	return nil
}

func sequenceValues(value object.Object) ([]object.Object, error) {
	switch v := value.(type) {
	case *object.List:
		return v.Elts, nil
	case *object.Tuple:
		return v.Elts, nil
	}
	it, err := object.Iterate(value)
	if err != nil {
		return nil, object.Raisef(object.TypeErrorClass, "cannot unpack non-iterable %s object", value.TypeName())
	}
	var out []object.Object
	for {
		v, err := it.Next()
		if err == object.ErrIterationDone {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
}

func (i *Interpreter) deleteTarget(target parser2.Expr, env *Env) error {
	switch t := target.(type) {
	case *parser2.Name:
		if !env.Delete(t.Id) {
			return object.Raisef(object.NameError, "name '%s' is not defined", t.Id)
		}
		return nil
	case *parser2.Subscript:
		obj, err := i.evalExpr(t.Value, env)
		if err != nil {
			return err
		}
		key, err := i.evalSubscriptKey(t.Slice, env)
		if err != nil {
			return err
		}
		return object.DelItem(obj, key)
	case *parser2.Attribute:
		return object.Raisef(object.NotImplementedError, "deleting attributes is not supported")
	}
	return object.Raisef(object.SyntaxErrorClass, "cannot delete this expression")
}
