# gpython

a simple and limited python 3.14 interpreter in go

## Overview

gpython is a small, pure-Go (no cgo) Python 3.14 interpreter that tree-walks
the `parser2` AST (`github.com/tamnd/gopapy/v2`) directly — no bytecode, no Go
code generation. It interprets the AST with an environment-based evaluator and
returns Python exceptions as ordinary Go errors.

## Build & run

Requires Go 1.26+.

```sh
go build ./...              # normal build
CGO_ENABLED=0 go build ./...  # verify no cgo
go vet ./...
go test ./...
gofmt -l .
```

Run the acceptance demo:

```sh
go run . demo.py
```

With no arguments, `go run .` enters the REPL (`>>>` / `...` prompts; press
Ctrl-D to exit with code 0). To run a script, pass its path explicitly:

```sh
go run . myscript.py
```

Exit codes: `0` success, `1` Python/runtime/syntax error, `2` usage error.

## Implemented language features

- Statements: `import` (3-level), `from ... import`, `class`, `def` (with
  `return`/`yield`), `try/except/else/finally`, `if/while/for`,
  `global`/`nonlocal`, `assert`, `del`, `raise`, `pass`, `break`/`continue`.
- Expressions: lambdas, ternary, f-strings, raw strings, `%` formatting,
  list/set/dict comprehensions, generator expressions, walrus, starred,
  attributes, subscripts, slices, all CPython comparison and binary operators.
- Exceptions: `try/except`, user-raised exceptions, lowercase interpreter error
  text (user-supplied messages preserved), `notimplementederror` for
  unimplemented features.
- `async`/`await` is **not** supported: any occurrence is fatal (exit 1).

## Standard library (`module/`)

Native modules registered in `module/registry.go`:

- `sys` — `argv`, `path`, `maxsize`, `modules`, `version`, `platform`, `exit`,
  `sys.modules` (a dict mirroring the interpreter module cache).
- `os` / `os.path` — `environ`, `getcwd`, `listdir`, `mkdir`, `remove`,
  `os.path.join`, `exists`, `isfile`, `isdir`, `basename`, `dirname`,
  `abspath`.
- `pathlib` — `Path`/`PurePath`/`cwd()`, `/` operator, `name`/`parent`/
  `suffix`/`stem`, `exists`/`is_file`/`is_dir`, `joinpath`, `as_posix`,
  `absolute`, `resolve`.
- `math` — `floor`/`ceil`/`trunc`/`sqrt`/`fabs`/`pow`/`exp`/`log`/
  `factorial`/`gcd`, `isnan`/`isinf`/`isfinite`, `sin`/`cos`/`tan`,
  `radians`/`degrees`, `pi`/`e`/`tau`/`inf`/`nan`.
- `this` — Zen of Python (ROT13), printed on import (`import this` / `s`/`d`/`c`).
- `threading` — `Thread` (`start`/`join`/`is_alive`/`name`/`daemon`),
  `Lock` (`acquire`/`release`/`locked`, with `blocking`/`timeout`),
  `RLock` (reentrant), `Event` (`set`/`clear`/`wait`/`is_set`),
  `active_count`, `current_thread`.
- `subprocess` — `run(args, capture_output=True)` returning a
  `CompletedProcess` (`returncode`/`stdout`/`stderr`/`args`), and
  `Popen(args)` with `poll`/`wait`/`communicate`.

`gpython_modules/` is supported as a package directory (`__init__.py` handled
in both the dependency and local lookup layers).

## Builtins

`print`, `len`, `isinstance`, `issubclass`, `repr`, `abs`, `min`, `max`, `sum`,
`sorted`, `enumerate`, `zip`, `next`, `iter`, `super`, `callable`, `hasattr`,
`getattr`/`setattr`, `ord`/`chr`/`round`/`hash`, plus `Ellipsis`, the exception
classes and `None`/`True`/`False`.

## Design notes

- Exceptions and control flow (`return`/`break`/`continue`) propagate as
  Go errors; `FatalError` bypasses `try` handlers. Generator bodies run in a
  dedicated goroutine and suspend at each `yield` via channels.
- Objects implement `object.Object` (`TypeName`/`Repr`/`Str`/`Truthy`);
  exceptions are `*object.Instance` with `IsException`.
- Builtin type names (`int`, `str`, `list`, …) are `*object.Class` values;
  constructors are dispatched in `call.go` by `BuiltinKind`.
- Zero-arg `super()` is resolved via the interpreter frame (`DefClass`).

See `DESIGN.md` for authoritative semantics and error formats, `TODO.md` for
milestone status, and `agent.md` for workflow and project conventions.
