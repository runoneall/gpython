package module

import (
	"os"
	"path/filepath"
	"runtime"

	"gpython/internal/object"
)

func init() {
	Register("sys", newSysModule)
}

// CacheSysModule records a loaded module in both the sys.modules map
// and the Python-visible dictionary.
func CacheSysModule(name string, m *object.Module) {
	SysModules[name] = m
	if SysModulesDict != nil {
		_ = SysModulesDict.Set(object.NewStr(name), m)
	}
}

// ForgetSysModule removes a failed module from the cache.
func ForgetSysModule(name string) {
	delete(SysModules, name)
	if SysModulesDict != nil {
		_, _ = SysModulesDict.Delete(object.NewStr(name))
	}
}

func newSysModule() *object.Module {
	m := object.NewModule("sys")

	argv := make([]object.Object, len(Argv))
	for i, a := range Argv {
		argv[i] = object.NewStr(a)
	}
	m.Dict["argv"] = object.NewList(argv...)

	paths := make([]object.Object, 0, 8)
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths, object.NewStr(wd))
	}
	for _, p := range filepath.SplitList(os.Getenv("GPYTHONPATH")) {
		if p != "" {
			paths = append(paths, object.NewStr(p))
		}
	}
	m.Dict["path"] = object.NewList(paths...)

	m.Dict["maxsize"] = object.NewInt(9223372036854775807)
	m.Dict["version"] = object.NewStr(Version)
	m.Dict["version_info"] = object.NewTuple(
		object.NewInt(3), object.NewInt(14), object.NewInt(0),
		object.NewStr("final"), object.NewInt(0),
	)
	m.Dict["platform"] = object.NewStr(runtime.GOOS)
	m.Dict["byteorder"] = object.NewStr(byteOrder())
	m.Dict["modules"] = SysModulesDict
	m.Dict["executable"] = object.NewStr("gpython")

	m.Dict["exit"] = object.NewBuiltin("exit", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(kwargs) != 0 || len(args) > 1 {
			return nil, object.Raisef(object.TypeErrorClass, "exit() takes at most 1 argument")
		}
		if len(args) == 0 {
			return nil, object.Raise(object.SystemExit, object.None)
		}
		return nil, object.Raise(object.SystemExit, args[0])
	})

	m.Dict["intern"] = object.NewBuiltin("intern", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 1 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "intern() takes exactly 1 argument")
		}
		if _, ok := args[0].(*object.Str); !ok {
			return nil, object.Raisef(object.TypeErrorClass, "intern() argument must be str")
		}
		return args[0], nil
	})

	return m
}

func byteOrder() string {
	// gpython only targets environments where this is cheap to detect.
	var x uint16 = 0x0102
	if byte(x&0xff) == 0x02 {
		return "little"
	}
	return "big"
}
