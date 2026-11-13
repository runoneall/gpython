package module

import (
	"os"
	"path/filepath"
	"sort"

	"gpython/internal/object"
)

func init() {
	Register("os", newOSModule)
	Register("os.path", newOSPathModule)
}

func newOSModule() *object.Module {
	m := object.NewModule("os")
	m.Dict["sep"] = object.NewStr(string(os.PathSeparator))
	m.Dict["name"] = object.NewStr("posix")
	m.Dict["linesep"] = object.NewStr("\n")
	m.Dict["path"] = newOSPathModule()

	env := object.NewDict()
	for _, kv := range os.Environ() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				_ = env.Set(object.NewStr(kv[:i]), object.NewStr(kv[i+1:]))
				break
			}
		}
	}
	m.Dict["environ"] = env

	m.Dict["getcwd"] = object.NewBuiltin("getcwd", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "getcwd() takes no arguments")
		}
		dir, err := os.Getwd()
		if err != nil {
			return nil, object.Raisef(object.OSError, "cannot determine current directory")
		}
		return object.NewStr(dir), nil
	})
	m.Dict["getenv"] = object.NewBuiltin("getenv", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, object.Raisef(object.TypeErrorClass, "getenv() takes 1 or 2 arguments")
		}
		name, ok := args[0].(*object.Str)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "getenv() argument must be str")
		}
		v, ok := os.LookupEnv(name.V)
		if !ok {
			if len(args) == 2 {
				return args[1], nil
			}
			return object.None, nil
		}
		return object.NewStr(v), nil
	})
	m.Dict["listdir"] = object.NewBuiltin("listdir", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		dir := "."
		if len(args) == 1 {
			s, ok := args[0].(*object.Str)
			if !ok {
				return nil, object.Raisef(object.TypeErrorClass, "listdir() argument must be str")
			}
			dir = s.V
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, object.Raisef(object.OSError, "no such file or directory: '%s'", dir)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		out := make([]object.Object, len(names))
		for i, n := range names {
			out[i] = object.NewStr(n)
		}
		return object.NewList(out...), nil
	})
	m.Dict["mkdir"] = object.NewBuiltin("mkdir", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 1 {
			return nil, object.Raisef(object.TypeErrorClass, "mkdir() takes exactly 1 argument")
		}
		s, ok := args[0].(*object.Str)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "mkdir() argument must be str")
		}
		if err := os.Mkdir(s.V, 0o777); err != nil {
			return nil, object.Raisef(object.OSError, "cannot create directory '%s'", s.V)
		}
		return object.None, nil
	})
	m.Dict["remove"] = object.NewBuiltin("remove", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 1 {
			return nil, object.Raisef(object.TypeErrorClass, "remove() takes exactly 1 argument")
		}
		s, ok := args[0].(*object.Str)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "remove() argument must be str")
		}
		if err := os.Remove(s.V); err != nil {
			return nil, object.Raisef(object.OSError, "cannot remove '%s'", s.V)
		}
		return object.None, nil
	})
	return m
}

func newOSPathModule() *object.Module {
	m := object.NewModule("os.path")
	m.Dict["sep"] = object.NewStr(string(os.PathSeparator))
	m.Dict["join"] = object.NewBuiltin("join", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			s, ok := a.(*object.Str)
			if !ok {
				return nil, object.Raisef(object.TypeErrorClass, "join() arguments must be str")
			}
			parts[i] = s.V
		}
		return object.NewStr(filepath.Join(parts...)), nil
	})
	m.Dict["exists"] = object.NewBuiltin("exists", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		s, err := oneString("exists", args, kwargs)
		if err != nil {
			return nil, err
		}
		_, statErr := os.Stat(s)
		return object.BoolFrom(statErr == nil), nil
	})
	m.Dict["isfile"] = object.NewBuiltin("isfile", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		s, err := oneString("isfile", args, kwargs)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Stat(s)
		return object.BoolFrom(statErr == nil && !info.IsDir()), nil
	})
	m.Dict["isdir"] = object.NewBuiltin("isdir", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		s, err := oneString("isdir", args, kwargs)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Stat(s)
		return object.BoolFrom(statErr == nil && info.IsDir()), nil
	})
	m.Dict["basename"] = object.NewBuiltin("basename", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		s, err := oneString("basename", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.NewStr(filepath.Base(s)), nil
	})
	m.Dict["dirname"] = object.NewBuiltin("dirname", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		s, err := oneString("dirname", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.NewStr(filepath.Dir(s)), nil
	})
	m.Dict["abspath"] = object.NewBuiltin("abspath", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		s, err := oneString("abspath", args, kwargs)
		if err != nil {
			return nil, err
		}
		abs, aerr := filepath.Abs(s)
		if aerr != nil {
			return nil, object.Raisef(object.OSError, "cannot resolve path '%s'", s)
		}
		return object.NewStr(abs), nil
	})
	return m
}

func oneString(name string, args []object.Object, kwargs map[string]object.Object) (string, error) {
	if len(args) != 1 || len(kwargs) != 0 {
		return "", object.Raisef(object.TypeErrorClass, "%s() takes exactly 1 argument", name)
	}
	s, ok := args[0].(*object.Str)
	if !ok {
		return "", object.Raisef(object.TypeErrorClass, "%s() argument must be str", name)
	}
	return s.V, nil
}
