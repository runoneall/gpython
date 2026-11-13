# DEVELOPMENT.md —— 如何扩展 gpython

本文档说明如何为 gpython 添加新功能，重点介绍两种扩展方式：

1. **标准库（`module/`）**：用 Go 编写，随解释器一起编译，通过 `init()` 注册。
2. **项目依赖（`gpython_modules/`）**：用 Python（`.py`）编写，无需重新编译解释器。

> 阅读本文档前，建议先浏览 `README.md`、`DESIGN.md`、`agent.md`，了解项目目标、架构和编码约定。

---

## 1. 概述：两种扩展方式的区别

| 维度           | 标准库 `module/`                         | 项目依赖 `gpython_modules/`                           |
| -------------- | ---------------------------------------- | ----------------------------------------------------- |
| 实现语言       | Go                                       | Python（`.py`，由 `parser2` 解析，tree-walking 执行） |
| 是否需重新编译 | 是（`go build` / `go run`）              | 否（直接读取源码执行）                                |
| 注册方式       | `init()` 调用 `module.Register`          | 无需注册，按文件路径自动发现                          |
| 导入优先级     | 最高（第一级）                           | 第二级                                                |
| 适用场景       | 需要调用 Go 标准库、高性能、底层系统接口 | 纯 Python 逻辑、快速迭代、无需编译 Go                 |
| 依赖限制       | 只能依赖 `internal/object`，避免循环依赖 | 可使用解释器已实现的全部 Python 语法                  |

导入时的三级查找顺序（`internal/interp/import.go`）：

```text
1. module/ 注册表（Go 标准库：this/os/sys/pathlib/math/threading/subprocess/…）
2. gpython_modules/ 目录下的 .py（项目依赖，识别 __init__.py 作为包）
3. 脚本目录 / 工作目录下的本地 .py（识别 __init__.py 作为包，可通过 GPYTHONPATH 追加）
```

---

## 2. 方式一：编写标准库（`module/`）

### 2.1 原理

`module/` 包下的每个 Go 文件都在 `func init()` 中将模块工厂注册到 `module` 包的全局注册表。`main.go` 通过空导入 `import _ "gpython/module"` 触发所有文件的 `init()` 执行，从而完成全量注册。Python 脚本的 `import xxx` 会先在该注册表中查找，命中后调用工厂函数构造模块对象并缓存到 `sys.modules`。

### 2.2 核心接口（`module/registry.go`）

```go
// Factory 构造一个新的模块对象。每次 import 命中时会调用一次，
// 得到一个独立的模块实例（其 Dict 可安全地被该脚本修改）。
type Factory func() *object.Module

// Register 将名字 -> 工厂 的映射注册到标准库注册表。
func Register(name string, factory Factory)

// Get 从注册表查找工厂，返回 (factory, 是否存在)。
func Get(name string) (Factory, bool)
```

`object.Module` 的定义（`internal/object/object.go`）：

```go
type Module struct {
	Name      string
	Dict      map[string]Object   // 模块的命名空间，即 Python 里的 __dict__
	IsPackage bool
	File      string
	Package   string
	Dir       string
}

func NewModule(name string) *Module
```

### 2.3 可用的 object 构造函数（`internal/object/`）

常用构造函数（全部在 `internal/object` 包中）：

| 函数                                                            | 说明                                                      |
| --------------------------------------------------------------- | --------------------------------------------------------- |
| `object.NewInt(v int64) *Int`                                   | 构造整数（gpython 整数为 int64）                          |
| `object.NewFloat(v float64) *Float`                             | 构造浮点数                                                |
| `object.NewStr(s string) *Str`                                  | 构造字符串                                                |
| `object.NewBool(v bool) Bool`                                   | 构造布尔值（`object.True` / `object.False` 为预定义单例） |
| `object.NewList(elts ...Object) *List`                          | 构造列表                                                  |
| `object.NewTuple(elts ...Object) *Tuple`                        | 构造元组                                                  |
| `object.NewDict() *Dict`                                        | 构造字典（保持插入顺序）                                  |
| `object.NewSet() *Set`                                          | 构造集合                                                  |
| `object.NewModule(name string) *Module`                         | 构造模块对象                                              |
| `object.NewBuiltin(name string, fn ...) *Builtin`               | 构造 Go 实现的内置函数                                    |
| `object.NewException(class *Class, args ...Object) *Instance`   | 构造异常实例                                              |
| `object.Raise(class *Class, args ...Object) error`              | 抛出异常（返回 Go error，由解释器捕获）                   |
| `object.Raisef(class *Class, format string, args ...any) error` | 带格式化的异常                                            |

内置异常类变量（`internal/object/exception.go`）：`object.TypeErrorClass`、`object.ValueErrorClass`、`object.NameErrorClass`、`object.AttributeErrorClass`、`object.ImportErrorClass`、`object.ZeroDivisionErrorClass`、`object.NotImplementedError`、`object.RuntimeError`、`object.SystemExit` 等。

### 2.4 逐步编写示例：新增一个 `hello` 标准库

假设要添加一个 `hello` 模块，提供 `say(name)` 函数和 `greeting` 变量。

#### 第一步：创建文件 `module/hello.go`

```go
package module

import "gpython/internal/object"

func init() {
	Register("hello", newHelloModule)
}

func newHelloModule() *object.Module {
	m := object.NewModule("hello")

	// 模块级变量
	m.Dict["greeting"] = object.NewStr("Hello, gpython!")

	// 内置函数：say(name)
	m.Dict["say"] = object.NewBuiltin("say", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 1 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "say() takes exactly 1 argument (%d given)", len(args))
		}
		name, ok := args[0].(*object.Str)
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "say() argument must be str, not '%s'", args[0].TypeName())
		}
		msg := "Hello, " + name.V + "!"
		// 可选：直接打印到标准输出（模拟 print）
		// 注意：标准库模块不直接导入 fmt 打印更推荐返回字符串，由 Python 层 print。
		return object.NewStr(msg), nil
	})

	return m
}
```

#### 第二步：确认 `main.go` 已空导入 `gpython/module`

`main.go` 应已包含：

```go
import (
	// ...
	"gpython/module"
)
```

不需要显式调用 `module` 包的任何函数，空导入即可触发所有 `init()`。

#### 第三步：构建并测试

```bash
CGO_ENABLED=0 go build ./...
go run . -c 'import hello; print(hello.say("world"))'   # 若支持 -c
# 或写一个 test_hello.py 运行：
# import hello
# print(hello.greeting)
# print(hello.say("gpython"))
```

预期输出：

```
Hello, gpython!
Hello, gpython!
```

### 2.5 编写内置函数的注意事项

1. **签名固定**：`func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error)`。`ctx` 用于调用用户可调用对象或获取 `super()`，当前模块若不需要可忽略。
2. **参数校验全部小写**：错误文案用 `object.Raisef(object.TypeErrorClass, "...")`，消息全部小写，符合 `DESIGN.md` 第 7 节。
3. **不要泄露 Go panic**：在内置函数内部避免 panic；如需保护，用 `defer/recover` 并转成 `object.Raisef`。
4. **依赖边界**：`module/` 只能依赖 `internal/object`，**不能**依赖 `internal/interp`，否则会形成循环依赖（`internal/interp` 依赖 `module`）。如需访问解释器状态（如当前帧、零参 `super()`），通过 `object.Caller` 接口。
5. **CGO**：任何新增的标准库文件都必须在 `CGO_ENABLED=0` 下能编译通过，即只使用 Go 标准库，不使用 cgo。

### 2.6 模块缓存辅助（`module/sys.go`）

`module` 包提供两个辅助函数，供解释器在加载模块时维护 `sys.modules`：

```go
func CacheSysModule(name string, m *object.Module)
func ForgetSysModule(name string)
```

标准库工厂通常**不直接调用**这两个函数，它们由 `internal/interp/import.go` 在加载后调用。编写标准库时只需关注 `Register` 和工厂函数即可。

---

## 3. 方式二：编写项目依赖（`gpython_modules/`）

### 3.1 原理

`gpython_modules/` 是项目级的 Python 模块依赖目录。其中的 `.py` 文件由解释器在运行时直接读取源码、`parser2.ParseFile` 解析并 tree-walking 执行，**无需重新编译 Go 程序**。它位于导入查找的第二级，仅在标准库中找不到时才会查找。

目录在 `.gitignore` 中被忽略（`gpython_modules/`），适合存放私有或生成的依赖，不提交到版本控制。

### 3.2 目录结构

```
<项目根>/gpython_modules/
├── foo.py                # 模块 foo，import foo 可用
├── bar.py                # 模块 bar
└── pkg/                  # 包 pkg，import pkg 可用
    ├── __init__.py       # 包的入口，作为模块 pkg 的命名空间
    └── sub.py            # 子模块，import pkg.sub 可用
```

### 3.3 编写示例

#### 单文件模块：`gpython_modules/hello.py`

```python
# gpython_modules/hello.py
GREETING = "Hello from gpython_modules!"


def say(name):
    return GREETING + " " + name
```

使用：

```python
import hello
print(hello.GREETING)      # Hello from gpython_modules!
print(hello.say("gpython"))  # Hello from gpython_modules! gpython
```

#### 包：`gpython_modules/pkg/__init__.py`

```python
# gpython_modules/pkg/__init__.py
from .sub import helper

__all__ = ["helper"]
```

```python
# gpython_modules/pkg/sub.py
def helper():
    return "pkg.helper"
```

使用：

```python
import pkg
print(pkg.helper())   # pkg.helper
from pkg.sub import helper
print(helper())       # pkg.helper
```

### 3.4 查找规则与 GPYTHONPATH

`internal/interp/import.go` 的 `findModuleFile` 按以下顺序查找 `.py` 文件或 `__init__.py`：

1. 脚本所在目录下的 `gpython_modules/`
2. 当前工作目录下的 `gpython_modules/`
3. `GPYTHONPATH` 环境变量中每个路径下的 `gpython_modules/`
4. 脚本所在目录（本地模块）
5. 当前工作目录（本地模块）
6. `i.moduleDir`（解释器配置的模块目录）
7. `GPYTHONPATH` 中每个路径（本地模块）

使用 `GPYTHONPATH` 示例：

```bash
# 将 /path/to/mydeps 加入查找路径，其下的 gpython_modules/ 会被检索
GPYTHONPATH=/path/to/mydeps go run . myscript.py
```

也可直接把依赖放在脚本同级目录下，无需设置环境变量。

### 3.5 与标准库的关系

- 同名时，**标准库优先**。例如 `gpython_modules/os.py` 不会覆盖 Go 实现的 `os` 标准库。
- `gpython_modules/` 内的 `.py` 模块可以 `import` 标准库模块（如 `import sys`），也可以 `import` 其他 `gpython_modules/` 下的模块。
- 循环导入会返回部分初始化的模块（占位符先写入 `sys.modules`，再执行模块体）。

---

## 4. 三级导入查找顺序速查

执行 `import a.b.c` 时：

1. `module.Get("a.b.c")` → 命中则直接使用 Go 标准库工厂构造。
2. 依次在 `gpython_modules/` 及各 `GPYTHONPATH` 下的 `gpython_modules/` 查找 `a/b/c.py` 或 `a/b/c/__init__.py`。
3. 依次在脚本目录、工作目录等本地路径查找 `a/b/c.py` 或 `a/b/c/__init__.py`。
4. 全部未命中 → 抛出 `importerror: no module named 'a.b.c'`（全小写）。

模块加载后会缓存到 `sys.modules`（`module.SysModules` 和 `module.SysModulesDict`），重复 `import` 不会重复执行。

---

## 5. 常见问题与注意事项

| 问题                                         | 说明                                                                                                                                |
| -------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| 标准库中能否导入 `internal/interp`？         | **不能**。`module/` 只能依赖 `internal/object`，避免循环依赖。                                                                      |
| 错误消息是否必须小写？                       | **是**。`DESIGN.md` 第 7 节要求解释器生成的所有错误信息全小写。用户传入的异常消息保持原样。                                         |
| 新增标准库后是否需要修改 `main.go`？         | 无需修改，只要 `main.go` 已有 `import _ "gpython/module"`，新文件的 `init()` 会自动触发。                                           |
| `gpython_modules/` 中的文件会被 git 忽略吗？ | 是，`.gitignore` 中有 `gpython_modules/`，不会提交到版本控制。                                                                      |
| 如何调试标准库模块？                         | 编写一个 `.py` 脚本 `import` 该模块并打印其属性，用 `go run . script.py` 运行；或在 `module/` 下添加 `*_test.go` 进行 Go 单元测试。 |
| 整数精度限制                                 | gpython 整数为 `int64`，超出范围的整数字面量会在解析阶段报错。                                                                      |
| 复数 / 字节串                                | 第一版明确不支持 `complex` 和 `bytes` 的特殊语义，`complex` 解析会报 `notimplementederror`，`b"..."` 按字符串处理。                 |

---

## 6. 快速检查清单

新增标准库模块后：

- [ ] 文件位于 `module/`，`package module`
- [ ] `func init()` 中调用了 `Register("modname", factory)`
- [ ] 工厂签名为 `func() *object.Module`
- [ ] 使用 `object.NewModule(name)` 创建模块，属性存入 `m.Dict`
- [ ] 内置函数使用 `object.NewBuiltin(name, func(ctx object.Caller, args, kwargs) (object.Object, error))`
- [ ] 错误消息全部小写
- [ ] 仅依赖 `internal/object` 及 Go 标准库（无 cgo）
- [ ] `CGO_ENABLED=0 go build ./...` 成功
- [ ] `go run . demo.py` 退出码 0，输出与期望一致
- [ ] `gofmt -l .`、`go vet ./...`、`go test ./...` 通过

新增 `gpython_modules/` 依赖后：

- [ ] `.py` 文件放在 `gpython_modules/` 或其子目录（包需含 `__init__.py`）
- [ ] 无需重新编译，直接用 `go run . script.py` 测试
- [ ] 如需跨目录，通过 `GPYTHONPATH` 指定额外查找路径
