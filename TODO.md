# TODO —— gpython 分阶段实现计划

本文件把 `DESIGN.md` 中的需求拆成可独立交付的里程碑，供多次对话逐步实现。
每完成一个里程碑，勾选任务、更新顶部进度表，并在文末“进度日志”追加一条记录。

配套文件：

- `DESIGN.md`：需求与设计依据（已固化）。
- `agent.md`：跨对话的工作约定、架构说明与交接协议。
- `demo.py`：最终验收脚本，文件末尾注释是期望输出。

---

## 进度总览

| 里程碑 | 内容                              | 状态       | 依赖        |
| ------ | --------------------------------- | ---------- | ----------- |
| M0     | 项目骨架、依赖、包布局            | [x] 已完成 | —           |
| M1     | 运行时对象模型                    | [x] 已完成 | M0          |
| M2     | 求值核心（表达式 + 基础语句）     | [x] 已完成 | M1          |
| M3     | 函数、闭包、装饰器、生成器        | [x] 已完成 | M2          |
| M4     | 类、继承、异常与 try/except       | [x] 已完成 | M3          |
| M5     | 字符串：f-string / r-string / `%` | [x] 已完成 | M2          |
| M6     | 内置函数与全小写错误体系          | [x] 已完成 | M2          |
| M7     | async/await 致命预检              | [x] 已完成 | M2          |
| M8     | 标准库 `module/` 注册表（this/os/sys/pathlib/math/threading/subprocess） | [x] 已完成 | M1、M6      |
| M9     | 三级 import 系统                  | [x] 已完成 | M3、M8      |
| M10    | CLI、traceback 与退出码           | [x] 已完成 | M4、M6      |
| M11    | 测试与验收（`demo.py` 全绿）      | [x] 已完成 | M5、M9、M10 |
| M12    | 收尾、文档、性能与清理            | [~] 进行中 | M11         |

状态取值：`[ ] 未开始`、`[~] 进行中`、`[x] 已完成`。

依赖图：

```text
M0 -> M1 -> M2 -> M3 -> M4 ----------------\
              \-> M5 ----------------------> M11 -> M12
              \-> M6 -> M8 -> M9 ----------/
              \-> M7 ---------------------/
M4 -> M10 ---------------------------------/
```

---

## M0 项目骨架、依赖、包布局

状态：[x] 已完成

目标：让 `go build ./...`、`CGO_ENABLED=0 go build ./...`、`go run .` 能跑通空解释器。

交付物：

- `go.mod` 增加 `github.com/tamnd/gopapy/v2 v2.0.0` 与 `go.sum`。
- `main.go`：CLI 入口骨架（读取参数、准备退出码）。
- `internal/object/`、`internal/interp/`、`internal/builtins/`、`module/` 目录与占位文件。
- `internal/interp/errors.go`：统一错误类型（可捕获错误 vs 致命错误）。

任务：

- [ ] `go get github.com/tamnd/gopapy/v2@v2.0.0`，确认 `go.sum` 生成。
- [ ] 建立目录：`internal/object`、`internal/interp`、`internal/builtins`、`module`、`testdata`。
- [ ] `internal/interp/errors.go` 定义：
  - `RuntimeError`（对应 Python 异常，可捕获）；
  - `FatalError`（不可捕获，用于 async/await 与内部错误）；
  - `SyntaxError` 包装 parser2 的 `filename:line:col: message`。
- [ ] `main.go` 解析 `os.Args`，预留 `runFile(path)` 与 `os.Exit(code)`。
- [ ] 确认错误文案常量集中在少量文件，便于全小写检查。
- [ ] 运行 `go vet ./...`。

验收：

```bash
go build ./...
CGO_ENABLED=0 go build ./...
go run .            # 输出用法或运行默认 demo.py（见 DESIGN 第 8 节）
```

---

## M1 运行时对象模型

状态：[x] 已完成

目标：实现解释器使用的值类型与基本协议。

交付物：

- `internal/object/object.go`：`Object` 接口、类型名、`Repr`、`Str`、`Truthy`。
- 基础类型：`NoneType`、`Bool`、`Int`、`Float`、`Str`、`List`、`Tuple`、`Dict`、`Set`。
- 迭代协议：`Iterable`、`Iterator`、`Generator`。
- `internal/object/exception.go`：异常对象与层级。

任务：

- [ ] 定义 `Object` 接口（`TypeName() string`、`Repr() string`、`Str() string`、`Truthy() bool`）。
- [ ] 实现 `None`、`bool`、`int64`、`float64`、`string` 包装与字面量构造。
- [ ] 实现 `list`/`tuple`：下标、负索引、切片、`len`、迭代、相等比较。
- [ ] 实现 `dict`：插入顺序、key 查找/赋值/删除、迭代 key、相等比较。
- [ ] 实现 `set`：去重、`in`、迭代、相等比较。
- [ ] 实现算术与比较分派（`Add`/`Sub`/`Mult`/`Div`/`FloorDiv`/`Mod`/`Pow`、`Eq`/`Lt` 等）。
- [ ] 实现 `Repr`/`Str`，需与 CPython 对齐（`demo.py` 期望输出依赖它）。
- [ ] 实现 `Generator` 对象（M3 使用，这里先放骨架）。
- [ ] 单元测试：`internal/object/*_test.go`，覆盖 repr、切片、dict 顺序、比较。

验收：

```bash
go test ./internal/object/...
```

---

## M2 求值核心（表达式 + 基础语句）

状态：[x] 已完成

目标：能执行不含函数/类/导入的脚本。

交付物：

- `internal/interp/interp.go`：`Interpreter`、模块执行入口。
- `internal/interp/env.go`：作用域链、`global`/`nonlocal`。
- `internal/interp/expr.go`、`stmt.go`、`control.go`。

任务：

- [ ] 从 `parser2.ParseFile` 结果构造模块对象与全局作用域。
- [ ] 作用域：Module/Function/Class 命名空间 + 内置作用域；LEGB 查找。
- [ ] 表达式求值：
  - [ ] `Constant`（映射 `Kind`/`Value`，注意 int64/float/str/None/True/False/Ellipsis）。
  - [ ] `Name`（未定义报 `nameerror`）。
  - [ ] `UnaryOp`、`BinOp`、`BoolOp`（短路）、`Compare`（链式比较）。
  - [ ] `Attribute`、`Subscript`、`Slice`。
  - [ ] `Call`（含 `*args`、`**kwargs` 解包）。
  - [ ] `List`/`Tuple`/`Set`/`Dict`。
  - [ ] `IfExp`（三元运算符）、`NamedExpr`（walrus）、`Starred`。
  - [ ] `Lambda`（复用 M3 的参数绑定，先接骨架）。
- [ ] 语句求值：
  - [ ] `ExprStmt`、`Assign`（多目标、元组解包）、`AugAssign`、`AnnAssign`。
  - [ ] `If`、`While`、`For`（含 `else`）、`Pass`、`Break`、`Continue`。
  - [ ] `Return`、`Delete`、`Assert`。
  - [ ] `Global`、`Nonlocal`。
- [ ] 控制流信号：`returnSignal`、`breakSignal`、`continueSignal`、`raiseSignal`，用 `panic/recover` 或显式返回实现其一并全项目统一。
- [ ] 预检：函数外 `return`、循环外 `break`/`continue` 报小写错误。
- [ ] 单元测试：算术、作用域、循环、解包、链式比较。

验收：

```bash
go test ./internal/interp/...
go run . testdata/m2_basic.py    # 自建基础脚本
```

---

## M3 函数、闭包、装饰器、生成器

状态：[x] 已完成

目标：完整实现 `def` 与 `return`/`yield`。

交付物：

- `internal/object/function.go`：`Function`、`BoundMethod`、`Builtin`。
- `internal/interp/call.go`：参数绑定与调用。
- `internal/object/generator.go`：生成器运行时。

任务：

- [ ] `def` 求值：创建函数对象（名称、`*Arguments`、默认值、函数体、闭包环境、所属模块）。
- [ ] 参数绑定：仅位置 `/`、位置、默认值、`*args`、仅关键字、`**kwargs`；错误全部小写。
- [ ] `return`；无返回值返回 `None`。
- [ ] 递归、函数作为值、嵌套函数与闭包（读闭包变量）。
- [ ] 装饰器：按从下到上应用，支持函数装饰器。
- [ ] `lambda` 复用同一套参数绑定。
- [ ] `yield` / `yield from`：
  - [ ] 含 `yield` 的函数调用时返回生成器对象，函数体不立即执行。
  - [ ] 迭代协议：`__iter__` 返回自身，`__next__` 跑到下一个 `yield`。
  - [ ] 结束抛 `StopIteration`；支持 `for`、`list()`、`next()`。
  - [ ] 第一版不支持 `send`/`throw`/`close`，报小写 `notimplementederror`。
- [ ] 单元测试：参数绑定各种组合、闭包、装饰器、生成器。

验收：

```bash
go test ./internal/interp/... ./internal/object/...
go run . testdata/m3_functions.py
```

---

## M4 类、继承、异常与 try/except

状态：[x] 已完成

目标：实现 `class`、`try/except/else/finally`、异常体系。

交付物：

- `internal/object/class.go`：`Class`、`Instance`、MRO、属性查找。
- `internal/object/exception.go`：完整异常层级。
- `internal/interp/stmt.go` 中 `ClassDef`、`Try` 分支。

任务：

- [ ] `ClassDef`：类体独立命名空间，执行后转类属性；类装饰器。
- [ ] 实例化：`__init__` 调用，返回实例。
- [ ] 属性查找：实例 -> 类 -> 基类（MRO），方法绑定为 `BoundMethod`。
- [ ] 多继承与基础 MRO（建议实现 C3，至少单继承正确）。
- [ ] `super()` 基础用法（含零参形式）。
- [ ] 自定义异常：继承 `Exception`；异常对象保存 `args`、消息、cause。
- [ ] `try`：
  - [ ] 多 `except`、`except as name`、裸 `except`、`except (A, B)`。
  - [ ] `else`、`finally`。
  - [ ] `raise`、`raise ValueError`、裸 `raise`、`raise ... from ...`。
  - [ ] `finally` 与 `return`/`break`/`continue` 的覆盖规则。
  - [ ] `except ... as e` 结束后删除 `e`（可选）。
- [ ] 未捕获异常向上传播到最外层（M10 负责打印）。
- [ ] 单元测试：继承、方法绑定、`super()`、异常匹配顺序、finally 覆盖。

验收：

```bash
go test ./internal/interp/... ./internal/object/...
go run . testdata/m4_classes.py
```

---

## M5 字符串：f-string / r-string / `%`

状态：[x] 已完成

目标：实现三类字符串语法与格式说明符。

交付物：

- `internal/object/format.go`：共享的格式化渲染逻辑。
- `internal/interp/expr.go` 中 `JoinedStr`/`FormattedValue` 分支。

任务：

- [ ] `JoinedStr`：拼接 `Constant` 与 `FormattedValue`。
- [ ] `FormattedValue.Conversion`：`-1` 原样、`114` repr、`115` str、`97` ascii（第一版可退化为 repr）。
- [ ] `FormatSpec`（本身是 `JoinedStr`）：`[[fill]align][sign][#][0][width][,][.precision][type]`。
- [ ] 类型码：`s`、`d`、`f`、`e`、`g`、`x`、`X`、`o`、`b`、`%`。
- [ ] 原始字符串 `r""`：解析器已保留反斜杠，按普通字符串使用；`rf""`/`fr""` 正常插值。
- [ ] `%` 格式化（`BinOp{Op:"Mod"}`）：`%s`/`%r`/`%d`/`%f`/`%x`/`%%` 等，右侧单值/元组/`%(name)s` 映射。
- [ ] 错误全小写：`valueerror: unknown format code ...`、`typeerror: not enough arguments for format string`。
- [ ] 单元测试：`demo.py` 第 6 节涉及的所有格式。

验收：

```bash
go test ./internal/object/... ./internal/interp/...
go run . testdata/m5_strings.py
```

---

## M6 内置函数与全小写错误体系

状态：[x] 已完成

目标：提供 `demo.py` 所需内置函数，并统一错误文案。

交付物：

- `internal/builtins/builtins.go`：内置函数注册。
- `internal/builtins/exceptions.go`：内置异常类注册。

任务：

- [ ] 函数：`print`（多参数、`sep` 默认空格）、`len`、`range`、`str`、`int`、`float`、`bool`、`list`、`tuple`、`dict`、`set`、`isinstance`、`issubclass`、`type`、`repr`、`abs`、`min`、`max`、`sum`、`sorted`、`enumerate`、`zip`、`next`、`iter`。
- [ ] 常量/类：`None`、`True`、`False`、`Exception` 及 6.9 异常层级全部类。
- [ ] 错误文案集中管理并全部小写；用户传入的异常消息不改写。
- [ ] 单元测试：每个内置函数的正常与错误路径。

验收：

```bash
go test ./internal/builtins/...
```

---

## M7 async/await 致命预检

状态：[x] 已完成

目标：解析到 `async`/`await` 立即报致命错误，不可捕获。

交付物：

- `internal/interp/async_check.go`：`checkNoAsync(*parser2.Module) error`。

任务：

- [ ] 递归遍历 AST，识别 `AsyncFunctionDef`、`AsyncFor`、`AsyncWith`、`Await`、`Comprehension.IsAsync == true`。
- [ ] 命中返回 `FatalError`，格式 `file:line:col: notimplementederror: async/await is not supported`。
- [ ] 在模块求值前调用；导入 `.py` 模块时同样先调用。
- [ ] 确保该错误绕过所有 `try/except`（不进入 `raiseSignal`）。
- [ ] 单元测试：`async def`、`await`、`async for`、`async with` 各一例，以及 `try: await x` 无法捕获。

验收：

```bash
go test ./internal/interp/...
go run . testdata/m7_async.py; echo $?   # 应为 1
```

---

## M8 标准库 `module/` 注册表

状态：[x] 已完成

目标：`module/` 目录 + `init()` 注册 + `main.go` 全量编译。

交付物：

- `module/registry.go`：注册表（名字 -> 模块工厂）。
- `module/this.go`、`module/os.go`、`module/sys.go`、`module/pathlib.go`。
- `main.go` 增加 `import _ "gpython/module"`。

任务：

- [x] 定义模块工厂签名（与 `internal/object` 对接，避免循环依赖）。
- [x] 每个标准库文件 `package module` + `func init()` 注册自身。
- [x] `this`：最小可用（打印 Zen 的 ROT13 实现）。
- [x] `os`：`sep`、`getcwd`、`listdir`、`path` 子集（按需）。
- [x] `sys`：`argv`、`path`、`maxsize`、`modules`、`version`、`exit`。
- [x] `pathlib`：`Path` 基础（拼接、`str`、`exists`、`name`、`parent`）。
- [x] `main.go` 通过空导入触发注册；确认 `CGO_ENABLED=0` 构建成功。
- [ ] 单元测试：注册表查询、`sys.maxsize == 9223372036854775807`、`os.sep == "/"`。

验收：

```bash
CGO_ENABLED=0 go build ./...
go test ./module/...
```

---

## M9 三级 import 系统

状态：[x] 已完成

目标：标准库 -> `gpython_modules/` -> 项目本地，三级查找。

交付物：

- `internal/interp/import.go`。
- 模块缓存（`sys.modules`）。

任务：

- [x] 语法支持：`import m`、`import m as x`、`import a.b`、`from m import n`、`from m import n as x`、`from m import *`。
- [x] 查找顺序：
  - [x] 1. `module` 注册表（标准库，Go）。
  - [x] 2. `gpython_modules/` 下的 `.py`（不重新编译）。
  - [x] 3. 脚本目录 / 工作目录（可选 `GPYTHONPATH`）。
- [x] 文件模块执行：模块全局作用域即 `__dict__`；支持 `pkg/__init__.py` 基础包。
- [x] 相对导入 `from . import x`、`from ..pkg import y`（基础）。
- [x] 模块缓存与循环导入（返回部分初始化模块）。
- [x] 导入错误全小写：`importerror: no module named 'foo'`、`importerror: cannot import name 'bar' from 'foo'`。
- [ ] 单元测试 + `testdata/m9_imports/` 与 `gpython_modules/` 示例。

验收：

```bash
go test ./internal/interp/...
go run . testdata/m9_imports/main.py
```

---

## M10 CLI、traceback 与退出码

状态：[x] 已完成

目标：完整命令行行为与错误输出。

交付物：

- `main.go` 完成。
- 追踪信息由 `internal/object/exception.go` + `main.go` 统一生成。

任务：

- [x] `go run .` 默认运行 `demo.py`（不存在则打印用法）。
- [x] `go run . demo.py arg1`：`sys.argv == ["demo.py", "arg1"]`。
- [x] 语法错误：`file:line:col: <小写消息>`，退出码 1。
- [x] 运行时未捕获异常：打印小写 traceback，退出码 1。
- [x] 致命错误（async/await）：直接打印小写消息并退出 1，无 traceback、不可捕获。
- [x] 用法错误退出码 2。
- [x] 确保不泄露 Go panic 文本；`recover` 兜底转成小写内部错误。
- [ ] 端到端测试脚本。

验收：

```bash
go run . demo.py; echo $?          # 0
go run . testdata/m10_error.py; echo $?   # 1，且输出全小写
go run . ; echo $?                 # 默认行为
```

---

## M11 测试与验收（`demo.py` 全绿）

状态：[x] 已完成

目标：`demo.py` 输出与文件末尾期望完全一致。

交付物：

- `demo.py` 全绿（输出与末尾期望注释逐行一致）。
- Go 测试覆盖各模块（部分，见下方任务）。
- 可选：`testdata/` 下分阶段脚本。

任务：

- [x] 逐行比对 `demo.py` 输出与末尾期望注释（见 agent.md 的校验命令）。
- [x] 覆盖 `demo.py` 全部语法：import、def、return、yield、lambda、三元、class、super、try/except、f-string、r-string、`%`、推导式、生成器表达式。
- [ ] `go test ./...` 全绿（当前无测试文件，构建/vet 均通过）。
- [x] `CGO_ENABLED=0 go build ./...` 成功。
- [x] `go vet ./...` 无告警。
- [x] 记录未实现但明确报错的特性清单（async/await、complex、template strings、type() 3-arg）。

验收：

```bash
go test ./...
CGO_ENABLED=0 go build ./...
go run . demo.py
```

---

## M12 收尾、文档、性能与清理

状态：[~] 进行中

目标：工程化收尾。

任务：

- [x] 更新 `README.md`：用法、支持范围、明确不支持的项（async/await 等）；已随标准库提交扩写。
- [x] 复核 `DESIGN.md` 与实际实现一致，修正偏差。
- [x] 清理死代码、统一错误文案、`gofmt`。
- [ ] 补充 `testdata/` 与测试说明（注意 `.gitignore`）。
- [ ] 性能基线：简单脚本的启动与执行时间记录（可选）。
- [x] 最终更新本文件进度表与进度日志（本次整理）。

验收：

```bash
gofmt -l .
go vet ./...
go test ./...
CGO_ENABLED=0 go build ./...
go run . demo.py
```

---

## 全局验收标准（Definition of Done）

1. `go run .` 与 `go run . demo.py` 可运行，`demo.py` 输出与期望逐行一致，退出码 0。
2. 已实现：import（三级）、class、def（return/yield）、try/except、lambda、三元、f-string/r-string/`%`、推导式、生成器表达式、标准库 `this`/`os`/`sys`/`pathlib`。
3. `async`/`await` 解析到即致命报错，退出码 1，且 `try/except` 无法捕获。
4. 解释器生成的所有错误信息全小写。
5. 不使用 cgo；`CGO_ENABLED=0 go build ./...` 成功。
6. `go test ./...`、`go vet ./...` 通过。

---

## 已决问题（统一声明）

- `go run .` 无参数时：进入 REPL（逐行读取、累积多行语句、解析执行），不再默认运行 `demo.py`；`demo.py` 仍需显式指定 `go run . demo.py`。
- `gpython_modules/`：需要 `__init__.py` 包支持，标准库/本地/依赖三级查找均应识别 `rel/__init__.py`。
- `this`：行为与 CPython 标准库一致，import 时打印 Zen of Python（ROT13，已实现）。
- 标准库：`this`/`os`/`sys`/`pathlib`/`math`/`threading`/`subprocess` 已全部落地，并通过 `demo.py` 与 `module/*_test.go` 验收。

## 第二阶段待办（M12 后续）

### 已落地（无需再做）

- [x] `main.go`：无参数时进入 REPL（逐行/多行累积 `ParseFile`，`>>> ` / `... ` 提示，EOF 退出）。
- [x] `internal/interp/import.go`：`findModuleFile` 已对 `deps`（`gpython_modules/`）与 `locals` 统一检查 `rel/__init__.py`，包支持无需额外改动。
- [x] `module/math.go`：已存在并注册（floor/ceil/sqrt 等），`demo.py` 已通过。

### 标准库（第二阶段新增）

- [x] `module/threading.go`：`Thread`（启动/join/is_alive/name/daemon）、`Lock`、`RLock`、`Event`、`active_count`/`current_thread` 已落地；与 Go `sync` 对接，无 cgo。（已落地，见提交 8f78fb5）
- [x] `module/subprocess.go`：`run(args, capture_output=True)` 返回 `CompletedProcess`，`Popen`（poll/wait/communicate）已落地；通过 Go `os/exec` 调用外部命令。（已落地，见提交 8f78fb5）
- [x] `module/registry.go`：`threading`/`subprocess` 已通过 `init()` 注册，`main.go` 空导入 `gpython/module` 即全量生效。（已落地，见提交 8f78fb5）

### 测试与验收

- [x] 补充单元测试：覆盖 `internal/object`、`internal/interp`、`module/`（`sys.maxsize`、`os.sep`、`pathlib.Path`、`threading`、`subprocess` 基础行为）；`module/threading_test.go`、`module/subprocess_test.go` 已落地。（已落地，见提交 8f78fb5）
- [ ] 补充 `testdata/`：按阶段放置 `.py` 脚本与期望输出，注意 `.gitignore` 忽略 `*.py`（仅 `!demo.py` 放行），可使用 `.txt` 后缀或在 `.gitignore` 增加 `!testdata/*.py` 例外。
- [ ] 端到端验证脚本：`go run . testdata/xxx.py` 的 stdout/stderr/退出码断言。

### 工程化收尾（M12）

- [ ] 完善 `README.md`：用法、支持范围、明确不支持的项（async/await 等）、示例。
- [ ] 性能基线：简单脚本的启动与执行时间记录（可选）。
- [ ] 最终 `gofmt -l .`、`go vet ./...`、`go test ./...`、`CGO_ENABLED=0 go build ./...`、`go run . demo.py` 全部绿灯。

---

## 进度日志

> 每次对话结束追加一条：日期、完成里程碑、改动文件、执行的验证、下一步。

- 2026-11-13：完成需求调研与 `DESIGN.md`、`demo.py`、`TODO.md`、`agent.md` 编写；尚未开始 Go 实现。
- 2026-11-13：完成全部 Go 实现并验收。
  - 新增：`main.go`（CLI、参数、退出码、traceback、SystemExit、panic recover）。
  - 新增：`module/sys.go`（argv/path/maxsize/modules/version/platform/exit、`module.CacheSysModule`/`ForgetSysModule` 辅助）。
  - 新增：`module/pathlib.go`（`Path` 值类：`/`、name/parent/suffix/stem/exists/is_file/is_dir/joinpath/as_posix/absolute/resolve；`Path()`/`PurePath()`/`cwd()`）。
  - 新增：`module/this.go`（Zen of Python ROT13，`s`/`d`/`c`）。
  - 改动：`module/registry.go` 增加 `SysModulesDict`；`internal/interp/interp.go`/`import.go` 改用 `module.CacheSysModule`/`ForgetSysModule`。
  - 改动：`internal/object/class.go` 新增 `Divider` 接口；`internal/object/number.go` `BinaryOp` 支持 `Divider`（`pathlib.Path / str`）。
  - 修复：`internal/object/class.go` `computeMRO` 空序列 slice panic。
  - 验证：`gofmt -l .` 无输出；`go vet ./...` 无告警；`CGO_ENABLED=0 go build ./...` 成功；`go run . demo.py` 退出码 0 且 stdout 与 demo.py 行 473–531 期望输出 `diff` 一致；`async`/`无脚本/语法错误/运行时异常` 均按预期退出码并小写输出。
  - 下一步：补充单元测试与 `testdata/`；如有时间再完善 README。
- 2026-11-13：按用户声明落地未决问题决定并同步代码。
  - `main.go`：无参数时进入 REPL（`>>> `/`... ` 提示、EOF 退出、表达式 repr、多语句持久 env）；`go run . demo.py` / `go run . demo.py arg1` 保持不变。
  - 确认 `import.go` `findModuleFile` 已对 `gpython_modules/` 依赖层检查 `rel/__init__.py`（包支持已满足）。
  - 改动：`internal/interp/errors.go` 新增导出 `WrapParseError`；`internal/interp/interp.go` 新增 `EvalExpr` 与 `SetScriptDir`。
  - 验证：`gofmt -l .` 无输出；`go vet ./...` 无告警；`CGO_ENABLED=0 go build ./...` 成功；`go run . demo.py` 退出码 0 且输出与期望 `diff` 一致；REPL 表达式/多行/语法错误/EOF 均按预期。
- 2026-11-13：整理多 AI 遗留的文档/代码不一致；同步 TODO.md 与 agent.md，使 `math`/`threading`/`subprocess` 状态与代码一致（均已落地，见提交 8f78fb5）；验证 `gofmt -l .`、`go vet ./...`、`go test ./...`、`CGO_ENABLED=0 go build ./...`、`go run . demo.py` 全部绿灯；下一步：提交并 `git push` 到 origin/main。
- 2026-11-13：修复项目卫生问题。
  - 补全单元测试：新增 `internal/object/object_test.go`、`internal/interp/interp_test.go`、`internal/builtins/builtins_test.go`，使各包均有 `_test.go`（`module/` 原有）。
  - 填充 `testdata/`：新增 `m2_basic.py`、`m4_classes.py`、`m5_strings.py` 三阶段冒烟脚本；`.gitignore` 增加 `!testdata/*.py` 例外。
  - 统一 `TODO.md` 里程碑状态：M0–M7 正文状态改为 `[x] 已完成`，与头部进度表一致。
  - 固化依赖：运行 `go mod tidy`，`go.mod`/`go.sum` 干净无变化，已提交。
  - 验证：`gofmt -l .`、`go vet ./...`、`go test ./...`、`CGO_ENABLED=0 go build ./...`、`go run . demo.py` 及三个 `testdata/` 脚本全部绿灯。
