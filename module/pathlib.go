package module

import (
	"os"
	"path/filepath"
	"strings"

	"gpython/internal/object"
)

func init() {
	Register("pathlib", newPathlibModule)
}

// Path is a small pathlib-compatible value. It deliberately covers
// only the operations gpython programs need: joining, string
// conversion, exists/name/parent and a few close relatives.
type Path struct {
	P string
}

func NewPath(p string) *Path { return &Path{P: p} }

func (*Path) TypeName() string { return "path" }
func (p *Path) Repr() string   { return "PosixPath('" + p.P + "')" }
func (p *Path) Str() string    { return p.P }
func (*Path) Truthy() bool     { return true }

// Div implements the `/` operator for paths.
func (p *Path) Div(other object.Object) (object.Object, error) {
	s, ok := other.(*object.Str)
	if !ok {
		if q, ok := other.(*Path); ok {
			return NewPath(filepath.Join(p.P, q.P)), nil
		}
		return nil, object.Raisef(object.TypeErrorClass, "unsupported operand type(s) for /: 'path' and '%s'", other.TypeName())
	}
	return NewPath(filepath.Join(p.P, s.V)), nil
}

func (p *Path) GetAttr(name string) (object.Object, error) {
	switch name {
	case "name":
		return object.NewStr(filepath.Base(p.P)), nil
	case "suffix":
		return object.NewStr(filepath.Ext(p.P)), nil
	case "stem":
		base := filepath.Base(p.P)
		return object.NewStr(strings.TrimSuffix(base, filepath.Ext(base))), nil
	case "parent":
		return NewPath(filepath.Dir(p.P)), nil
	case "anchor":
		vol := filepath.VolumeName(p.P)
		if strings.HasPrefix(p.P, string(os.PathSeparator)) {
			return object.NewStr(vol + string(os.PathSeparator)), nil
		}
		return object.NewStr(vol), nil
	case "exists":
		return object.NewBuiltin("exists", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "exists() takes no arguments")
			}
			_, err := os.Stat(p.P)
			return object.BoolFrom(err == nil), nil
		}), nil
	case "is_file":
		return object.NewBuiltin("is_file", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "is_file() takes no arguments")
			}
			info, err := os.Stat(p.P)
			return object.BoolFrom(err == nil && !info.IsDir()), nil
		}), nil
	case "is_dir":
		return object.NewBuiltin("is_dir", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "is_dir() takes no arguments")
			}
			info, err := os.Stat(p.P)
			return object.BoolFrom(err == nil && info.IsDir()), nil
		}), nil
	case "joinpath":
		return object.NewBuiltin("joinpath", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "joinpath() takes no keyword arguments")
			}
			parts := []string{p.P}
			for _, a := range args {
				s, ok := a.(*object.Str)
				if !ok {
					return nil, object.Raisef(object.TypeErrorClass, "joinpath() arguments must be str")
				}
				parts = append(parts, s.V)
			}
			return NewPath(filepath.Join(parts...)), nil
		}), nil
	case "as_posix":
		return object.NewBuiltin("as_posix", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "as_posix() takes no arguments")
			}
			return object.NewStr(filepath.ToSlash(p.P)), nil
		}), nil
	case "absolute":
		return object.NewBuiltin("absolute", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "absolute() takes no arguments")
			}
			abs, err := filepath.Abs(p.P)
			if err != nil {
				return nil, object.Raisef(object.OSError, "cannot resolve path '%s'", p.P)
			}
			return NewPath(abs), nil
		}), nil
	case "resolve":
		return object.NewBuiltin("resolve", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "resolve() takes no arguments")
			}
			abs, err := filepath.Abs(p.P)
			if err != nil {
				return nil, object.Raisef(object.OSError, "cannot resolve path '%s'", p.P)
			}
			if real, err := filepath.EvalSymlinks(abs); err == nil {
				abs = real
			}
			return NewPath(abs), nil
		}), nil
	}
	return nil, object.Raisef(object.AttributeError, "'path' object has no attribute '%s'", name)
}

func (p *Path) SetAttr(name string, value object.Object) error {
	return object.Raisef(object.AttributeError, "'path' object attribute '%s' is read-only", name)
}

func newPathlibModule() *object.Module {
	m := object.NewModule("pathlib")
	pathCtor := object.NewBuiltin("Path", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "Path() takes no keyword arguments")
		}
		if len(args) == 0 {
			return NewPath("."), nil
		}
		parts := make([]string, len(args))
		for i, a := range args {
			switch v := a.(type) {
			case *object.Str:
				parts[i] = v.V
			case *Path:
				parts[i] = v.P
			default:
				return nil, object.Raisef(object.TypeErrorClass, "Path() arguments must be str or path")
			}
		}
		return NewPath(filepath.Join(parts...)), nil
	})
	m.Dict["Path"] = pathCtor
	m.Dict["PurePath"] = pathCtor

	m.Dict["cwd"] = object.NewBuiltin("cwd", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "cwd() takes no arguments")
		}
		wd, err := os.Getwd()
		if err != nil {
			return nil, object.Raisef(object.OSError, "cannot determine current directory")
		}
		return NewPath(wd), nil
	})
	return m
}
