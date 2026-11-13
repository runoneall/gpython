package interp

import (
	"os"
	"path/filepath"
	"strings"

	"gpython/internal/object"
	"gpython/module"

	"github.com/tamnd/gopapy/v2/parser2"
)

func (i *Interpreter) evalImport(n *parser2.Import, env *Env) error {
	for _, alias := range n.Names {
		mod, err := i.importModule(alias.Name)
		if err != nil {
			return err
		}
		if alias.Asname != "" {
			env.Set(alias.Asname, mod)
			continue
		}
		top := alias.Name
		if idx := strings.IndexByte(top, '.'); idx >= 0 {
			top = top[:idx]
		}
		if m, ok := i.Modules[top]; ok {
			env.Set(top, m)
		} else {
			env.Set(top, mod)
		}
	}
	return nil
}

func (i *Interpreter) evalImportFrom(n *parser2.ImportFrom, env *Env) error {
	name := n.Module
	if n.Level > 0 {
		resolved, err := i.resolveRelative(n.Level, n.Module)
		if err != nil {
			return err
		}
		name = resolved
	}
	mod, err := i.importModule(name)
	if err != nil {
		return err
	}
	for _, alias := range n.Names {
		if alias.Name == "*" {
			for k, v := range mod.Dict {
				if strings.HasPrefix(k, "_") {
					continue
				}
				env.Set(k, v)
			}
			continue
		}
		v, ok := mod.Dict[alias.Name]
		if !ok {
			full := alias.Name
			if mod.Name != "" {
				full = mod.Name + "." + alias.Name
			}
			sub, serr := i.importModule(full)
			if serr == nil {
				v = sub
				ok = true
			}
		}
		if !ok {
			return object.Raisef(object.ImportError, "cannot import name '%s' from '%s'", alias.Name, mod.Name)
		}
		if alias.Asname != "" {
			env.Set(alias.Asname, v)
		} else {
			env.Set(alias.Name, v)
		}
	}
	return nil
}

func (i *Interpreter) resolveRelative(level int, name string) (string, error) {
	pkg := i.moduleName
	if !i.moduleIsPackage {
		if idx := strings.LastIndexByte(pkg, '.'); idx >= 0 {
			pkg = pkg[:idx]
		} else {
			pkg = ""
		}
	}
	parts := []string{}
	if pkg != "" {
		parts = strings.Split(pkg, ".")
	}
	up := level - 1
	if up > len(parts) {
		return "", object.Raisef(object.ImportError, "attempted relative import beyond top-level package")
	}
	parts = parts[:len(parts)-up]
	base := strings.Join(parts, ".")
	if name == "" {
		return base, nil
	}
	if base == "" {
		return name, nil
	}
	return base + "." + name, nil
}

func (i *Interpreter) importModule(name string) (*object.Module, error) {
	if name == "" {
		return nil, object.Raisef(object.ImportError, "no module named ''")
	}
	if m, ok := i.Modules[name]; ok && m != nil {
		return m, nil
	}
	if factory, ok := module.Get(name); ok {
		m := factory()
		i.Modules[name] = m
		module.CacheSysModule(name, m)
		return m, nil
	}

	if idx := strings.LastIndexByte(name, '.'); idx >= 0 {
		parentName := name[:idx]
		childName := name[idx+1:]
		parent, err := i.importModule(parentName)
		if err != nil {
			return nil, err
		}
		if sub, ok := parent.Dict[childName]; ok {
			if m, ok := sub.(*object.Module); ok {
				i.Modules[name] = m
				module.CacheSysModule(name, m)
				return m, nil
			}
		}
	}

	file, isPkg, err := i.findModuleFile(name)
	if err != nil {
		return nil, err
	}
	m, err := i.loadFileModule(name, file, isPkg)
	if err != nil {
		return nil, err
	}
	if idx := strings.LastIndexByte(name, '.'); idx >= 0 {
		parentName := name[:idx]
		childName := name[idx+1:]
		if parent, ok := i.Modules[parentName]; ok {
			parent.Dict[childName] = m
		}
	}
	return m, nil
}

func (i *Interpreter) findModuleFile(name string) (string, bool, error) {
	rel := strings.ReplaceAll(name, ".", string(filepath.Separator))
	var deps []string
	var locals []string
	seen := map[string]bool{}
	add := func(dst *[]string, dir string) {
		if dir == "" {
			return
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		*dst = append(*dst, abs)
	}
	scriptDir := i.scriptDir
	cwd, _ := os.Getwd()
	for _, base := range []string{scriptDir, cwd} {
		if base != "" {
			add(&deps, filepath.Join(base, "gpython_modules"))
		}
	}
	for _, p := range filepath.SplitList(os.Getenv("GPYTHONPATH")) {
		add(&deps, filepath.Join(p, "gpython_modules"))
	}
	for _, base := range []string{scriptDir, cwd, i.moduleDir} {
		add(&locals, base)
	}
	for _, p := range filepath.SplitList(os.Getenv("GPYTHONPATH")) {
		add(&locals, p)
	}
	for _, dir := range append(deps, locals...) {
		file := filepath.Join(dir, rel+".py")
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			return file, false, nil
		}
		initFile := filepath.Join(dir, rel, "__init__.py")
		if info, err := os.Stat(initFile); err == nil && !info.IsDir() {
			return initFile, true, nil
		}
	}
	return "", false, object.Raisef(object.ImportError, "no module named '%s'", name)
}

func (i *Interpreter) loadFileModule(name, file string, isPkg bool) (*object.Module, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, object.Raisef(object.ImportError, "no module named '%s'", name)
	}
	parsed, err := parser2.ParseFile(file, string(src))
	if err != nil {
		return nil, wrapParseError(err)
	}
	// Cache a placeholder before execution to support circular imports.
	placeholder := object.NewModule(name)
	placeholder.File = file
	placeholder.IsPackage = isPkg
	i.Modules[name] = placeholder
	module.CacheSysModule(name, placeholder)
	if err := i.runModule(name, file, parsed, isPkg); err != nil {
		delete(i.Modules, name)
		module.ForgetSysModule(name)
		return nil, err
	}
	return i.Modules[name], nil
}
