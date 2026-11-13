// Package module implements the small native standard library used by
// gpython. Each source file registers its modules from init().
package module

import (
	"sync"

	"gpython/internal/object"
)

// Factory builds a fresh module object.
type Factory func() *object.Module

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}

	// SysModules mirrors sys.modules and is kept in sync by the
	// interpreter. SysModulesDict is the Python-visible dictionary
	// exposed as sys.modules.
	SysModules     = map[string]*object.Module{}
	SysModulesDict = object.NewDict()

	// Argv is installed by main before the script runs.
	Argv []string

	// Version is the string exposed as sys.version.
	Version = "3.14.0 (gpython)"
)

// Register adds a module factory to the native registry.
func Register(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// Get looks a module up in the native registry.
func Get(name string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[name]
	return f, ok
}
