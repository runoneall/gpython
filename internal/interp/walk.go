package interp

import (
	"reflect"

	"github.com/tamnd/gopapy/v2/parser2"
)

var posType = reflect.TypeOf(parser2.Pos{})

// walkAST visits every node in an AST subtree. Returning false from
// visit skips the children of that node. The pos passed to visit is the
// nearest enclosing source position; parser2.Comprehension has no Pos
// field of its own, so it inherits its parent's location.
func walkAST(node any, visit func(node any, pos parser2.Pos) bool) {
	walkValue(reflect.ValueOf(node), parser2.Pos{}, visit)
}

func walkValue(v reflect.Value, pos parser2.Pos, visit func(node any, pos parser2.Pos) bool) {
	if !v.IsValid() {
		return
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		if f := v.FieldByName("P"); f.IsValid() && f.Type() == posType {
			pos = f.Interface().(parser2.Pos)
		}
		var node any
		if v.CanAddr() {
			node = v.Addr().Interface()
		} else {
			node = v.Interface()
		}
		if !visit(node, pos) {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if !f.CanInterface() {
				continue
			}
			if f.Type() == posType {
				continue
			}
			walkValue(f, pos, visit)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			walkValue(v.Index(i), pos, visit)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			walkValue(iter.Value(), pos, visit)
		}
	}
}

// nodePos extracts the P field from an AST node.
func nodePos(node any) parser2.Pos {
	v := reflect.ValueOf(node)
	for v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return parser2.Pos{}
		}
		v = v.Elem()
	}
	if v.IsValid() && v.Kind() == reflect.Struct {
		if f := v.FieldByName("P"); f.IsValid() && f.Type() == posType {
			return f.Interface().(parser2.Pos)
		}
	}
	return parser2.Pos{}
}

// hasYield reports whether a function body contains a yield in its own
// scope, ignoring nested functions, lambdas and classes.
func hasYield(stmts []parser2.Stmt) bool {
	found := false
	visit := func(node any, _ parser2.Pos) bool {
		switch node.(type) {
		case *parser2.Yield, *parser2.YieldFrom:
			found = true
			return false
		case *parser2.FunctionDef, *parser2.AsyncFunctionDef, *parser2.Lambda, *parser2.ClassDef:
			return false
		}
		return !found
	}
	for _, s := range stmts {
		walkAST(s, visit)
		if found {
			return true
		}
	}
	return false
}
