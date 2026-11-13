# gpython 设计资料汇总

> 本文档是第一阶段的信息收集与设计固化结果。  
> 当前**没有开始实现解释器**，等待下一步指令后再编码。  
> 调研日期：2026-11-13。

---

## 1. 项目目标与硬性约束

| 项目     | 内容                                                                                |
| -------- | ----------------------------------------------------------------------------------- |
| 目标     | 一个 simple and limited 的 Python 3.14 解释器                                       |
| 语言     | Go（`go 1.26.8`）                                                                   |
| 解析器   | `github.com/tamnd/gopapy/v2`（`parser2`）                                           |
| CGO      | 不使用，必须能在 `CGO_ENABLED=0` 下构建                                             |
| 运行方式 | 解释执行（遍历 AST，不生成 Go 代码、不编译成字节码）                                |
| 入口     | `main.go`                                                                           |
| 启动命令 | `go run .`                                                                          |
| 必须实现 | `import` 导入、`class` 类、`def` 函数（含 `return`/`yield`）、`try/except` 错误处理 |
| 标准库   | 起始包含 `this`、`os`、`sys`、`pathlib`，用 Go 实现在 `module/` 目录                |
| 字符串   | 实现 f-string（`f""`）、原始字符串（`r""`）以及 `"%..."` 百分号格式化               |
| 表达式   | 实现 `lambda` 匿名函数与三元运算符 `a if cond else b`                               |
| 推导式   | 实现列表/集合/字典推导式与生成器表达式 `(...)`                                      |
| async    | 明确不实现 `async`/`await`，解析到就报致命错误并退出，且不可被 `try/except` 捕获    |
| 错误信息 | 解释器生成的所有错误信息统一使用小写                                                |

“simple and limited” 的含义：优先保证上述四类语法和常见程序可运行，不追求完整 CPython 兼容；不实现 async、元类、描述符等高级特性，遇到不支持的特性给出明确的小写错误。并发由 Go 的 goroutine 承担，不引入 Python 的 async/await 模型。

---

## 2. 仓库现状

```text
/home/PartXI/gpython
├── .gitignore
├── LICENSE
├── README.md       # 仅 3 行：a simple and limited python 3.14 interpreter in go
├── go.mod          # module gpython / go 1.26.8
├── main.go         # 只有 package main
└── demo.py         # 最终验收演示脚本（.gitignore 中例外保留）
```

其他事实：

- Git 工作区干净，最近提交为 `e2364c0 Add initial project files...`。
- `go.mod` 目前还没有 require `github.com/tamnd/gopapy/v2`。
- `.gitignore` 第 36 行忽略 `*.py`，第 37 行用 `!demo.py` 放行验收脚本；`testdata/` 下的 `.py` 仍需例外或改用 `.txt`。
- `go env`：`GOOS=linux`、`GOARCH=amd64`、`CGO_ENABLED=1`（环境默认值，不代表项目需要 cgo）。
- 模块缓存中已有 `github.com/tamnd/gopapy/v2@v2.0.0`，可以离线使用。

---

## 3. gopapy/v2 调研结果

### 3.1 依赖事实

```bash
go get github.com/tamnd/gopapy/v2@v2.0.0
```

- 最新版本：`v2.0.0`（2026-04-27 发布，MIT）。
- 只依赖 Go 标准库；`go list -deps` 显示非标准库依赖只有 `parser2` 本身。
- 已用最小程序验证：`CGO_ENABLED=0 go build` 成功。
- 它**只是解析器**，不提供求值器、对象模型、内置函数或运行时，所有执行语义都需要本项目实现。
- 官方说明：`parser2` 覆盖完整 CPython 3.14 语法（85/85 grammar fixtures）。

### 3.2 公开 API

```go
import "github.com/tamnd/gopapy/v2/parser2"

func ParseFile(filename, src string) (*Module, error)
func ParseString(filename, src string) (*Module, error) // ParseFile 的别名
func ParseExpression(src string) (Expr, error)
func DumpModule(m *Module) string
func Dump(e Expr) string
```

- `ParseFile` 的 `filename` 只用于错误信息前缀，源码内容来自 `src`。
- 语法错误格式为：`filename:line:col: message`，例如：
  `bad.py:1:4: expected :, got NEWLINE`。
- 位置 `Pos{Line, Col}` 从 1 开始，只有起点，没有结束位置。
- `ParseExpression` 的错误不带文件名，只带 `line:col`。

### 3.3 已验证的解析输出

以下源码（覆盖 import / def / class / try）可以被 `v2.0.0` 正确解析：

```python
import math
import os.path as osp
from collections import OrderedDict, defaultdict as dd

def add(a, b=2, *args, c=3, **kw):
    return a + b

class Animal:
    kind = "animal"
    def __init__(self, name):
        self.name = name
    def speak(self):
        return self.name + " speaks"

try:
    x = add(1)
    raise ValueError("bad") from None
except ValueError as e:
    print(e)
except (TypeError, KeyError):
    pass
else:
    x = 0
finally:
    x = -1
```

解析结果（`DumpModule` 实际输出）：

```text
Module(body=[Import(names=[Alias(name="math")]), Import(names=[Alias(name="os.path", asname="osp")]), ImportFrom(module="collections", names=[Alias(name="OrderedDict"), Alias(name="defaultdict", asname="dd")], level=0), FunctionDef(name="add", args=Arguments(args=[a, b], vararg=args, kwonly=[c], kwarg=kw), body=[Return(value=BinOp(op=Add, left=Name(id="a"), right=Name(id="b")))], decorators=[], returns=nil), ClassDef(name="Animal", bases=[], keywords=[], body=[Assign(targets=[Name(id="kind")], value=Constant(value="animal")), FunctionDef(name="__init__", args=Arguments(args=[self, name]), body=[Assign(targets=[Attribute(value=Name(id="self"), attr="name")], value=Name(id="name"))], decorators=[], returns=nil), FunctionDef(name="speak", args=Arguments(args=[self]), body=[Return(value=BinOp(op=Add, left=Attribute(value=Name(id="self"), attr="name"), right=Constant(value=" speaks")))], decorators=[], returns=nil)], decorators=[]), Try(body=[Assign(targets=[Name(id="x")], value=Call(func=Name(id="add"), args=[Constant(value=1)], keywords=[])), Raise(exc=Call(func=Name(id="ValueError"), args=[Constant(value="bad")], keywords=[]), cause=Constant(value=None))], handlers=[ExceptHandler(type=Name(id="ValueError"), name="e", body=[Expr(value=Call(func=Name(id="print"), args=[Name(id="e")], keywords=[]))]), ExceptHandler(type=Tuple([Name(id="TypeError"), Name(id="KeyError")]), name="", body=[Pass()])], orelse=[Assign(targets=[Name(id="x")], value=Constant(value=0))], finalbody=[Assign(targets=[Name(id="x")], value=UnaryOp(op=USub, operand=Constant(value=1)))])])
```

### 3.4 常量表示（实测）

| Python 写法      | `Constant.Kind`      | `Constant.Value` 的 Go 类型/内容 |
| ---------------- | -------------------- | -------------------------------- |
| `1`              | `"int"`              | `int64`                          |
| `1.5`            | `"float"`            | `float64`                        |
| `2j`             | `"complex"`          | `string`，内容 `"2j"`            |
| `b"hi"`          | `"bytes"`            | `string`，内容 `"hi"`            |
| `"x"`            | `"str"`              | `string`                         |
| `True` / `False` | `"True"` / `"False"` | `nil`                            |
| `None`           | `"None"`             | `nil`                            |
| `...`            | `"Ellipsis"`         | `nil`                            |

实现时需要在求值阶段把 `Kind`/`Value` 转成自己的运行时对象：

- 整数只有 `int64`，解析超大整数会在 parse 阶段报 `invalid int literal`。
- complex 只拿到源码字符串，建议第一版不支持 complex，报小写 `notimplementederror`。
- bytes 实际是 Go string，建议第一版按字符串处理或明确不支持。
- 原始字符串 `r"..."`：解析器原样保留反斜杠，`Kind` 仍是 `"str"`，解释器无需额外处理。
- f-string：`Kind` 为 `"str"` 的普通片段是 `JoinedStr.Values` 里的 `Constant`；插值是 `FormattedValue`，其 `Conversion` 为 `-1`/`114`(`!r`)/`115`(`!s`)/`97`(`!a`)，`FormatSpec` 是另一个 `JoinedStr` 或 nil；`rf""`/`fr""` 同样走 `JoinedStr`，只是普通片段不做转义。

### 3.5 运算符字段的取值

AST 中的 `Op` 是首字母大写的 CPython 风格名字，不是源码符号：

- 二元：`Add`、`Sub`、`Mult`、`Div`、`FloorDiv`、`Mod`、`Pow`、`MatMult`、`LShift`、`RShift`、`BitOr`、`BitXor`、`BitAnd`
- 一元：`UAdd`、`USub`、`Invert`、`Not`
- 布尔：`And`、`Or`
- 比较：`Eq`、`NotEq`、`Lt`、`LtE`、`Gt`、`GtE`、`Is`、`IsNot`、`In`、`NotIn`
- `AugAssign` 的 `Op` 同样是 `Add`/`Sub` 等名字。

### 3.6 解析器已知边界与坑

1. **不做语义检查**：`return` 在函数外、`break` 在循环外、`self` 未定义等语法树都能生成，必须由解释器在运行/校验阶段报错。
2. `try` 的 `except*`（PEP 654）被当作普通 `except` 解析，AST 中没有 `TryStar` 节点；第一版可明确不支持 `except*`。
3. `match`、`type` 别名、`with` 等都能解析，但不属于本阶段必须实现范围；f-string、`lambda`、三元运算符、推导式与生成器表达式已提升为必须实现；`async`/`await` 虽然能解析，但按项目约定直接判为致命错误。
4. AST 没有源码文本，错误信息只能依靠 `Pos` 和自行维护的文件名/行号。
5. `Arguments` 的默认值规则：
   - `Defaults` 对应 `Args` 末尾的 N 个参数；
   - `KwOnlyDef` 与 `KwOnly` 一一对应，`nil` 表示没有默认值；
   - `PosOnly`、`Args`、`Vararg`、`KwOnly`、`Kwarg` 分开存放。
6. 装饰器在 `DecoratorList` 中按源码顺序保存。
7. `AnnAssign` 已区分 `Simple`，注解表达式需要求值或忽略，建议第一版只忽略注解。
8. `ImportFrom.Level` 表示相对导入的点数；`Module` 可能为空（只有点）。

---

## 4. 需要实现的 AST 子集

下面按“必须 / 建议 / 暂缓”划分，便于后续编码。

### 4.1 语句节点（`parser2.Stmt`）

| Go 类型                                       | 字段                                                          | 第一版           |
| --------------------------------------------- | ------------------------------------------------------------- | ---------------- |
| `Module`                                      | `Body []Stmt`                                                 | 必须             |
| `ExprStmt`                                    | `Value Expr`                                                  | 必须             |
| `Assign`                                      | `Targets []Expr`、`Value Expr`                                | 必须             |
| `AugAssign`                                   | `Target Expr`、`Op string`、`Value Expr`                      | 必须             |
| `AnnAssign`                                   | `Target`、`Annotation`、`Value`、`Simple`                     | 建议（忽略注解） |
| `Return`                                      | `Value Expr`（可 nil）                                        | 必须             |
| `Raise`                                       | `Exc Expr`、`Cause Expr`（可 nil）                            | 必须             |
| `Pass` / `Break` / `Continue`                 | `P Pos`                                                       | 必须             |
| `Import`                                      | `Names []*Alias`                                              | 必须             |
| `ImportFrom`                                  | `Module string`、`Names []*Alias`、`Level int`                | 必须             |
| `Global` / `Nonlocal`                         | `Names []string`                                              | 建议             |
| `Delete`                                      | `Targets []Expr`                                              | 建议             |
| `Assert`                                      | `Test`、`Msg`                                                 | 建议             |
| `If`                                          | `Test`、`Body`、`Orelse`                                      | 必须             |
| `While`                                       | `Test`、`Body`、`Orelse`                                      | 必须             |
| `For`                                         | `Target`、`Iter`、`Body`、`Orelse`                            | 必须             |
| `Try`                                         | `Body`、`Handlers []*ExceptHandler`、`Orelse`、`Finalbody`    | 必须             |
| `ExceptHandler`                               | `Type Expr`（可 nil）、`Name string`、`Body []Stmt`           | 必须             |
| `FunctionDef`                                 | `Name`、`Args *Arguments`、`Body`、`DecoratorList`、`Returns` | 必须             |
| `ClassDef`                                    | `Name`、`Bases`、`Keywords`、`Body`、`DecoratorList`          | 必须             |
| `With`                                        | `Items []*WithItem`、`Body`                                   | 暂缓             |
| `AsyncFor` / `AsyncFunctionDef` / `AsyncWith` | —                                                             | 禁止（致命报错） |
| `TypeAlias`                                   | `Name`、`TypeParams`、`Value`                                 | 暂缓             |
| `Match`                                       | `Subject`、`Cases`                                            | 暂缓             |

### 4.2 表达式节点（`parser2.Expr`）

| Go 类型                                              | 字段                                           | 第一版                |
| ---------------------------------------------------- | ---------------------------------------------- | --------------------- |
| `Constant`                                           | `Kind`、`Value`                                | 必须                  |
| `Name`                                               | `Id`                                           | 必须                  |
| `UnaryOp`                                            | `Op`、`Operand`                                | 必须                  |
| `BinOp`                                              | `Op`、`Left`、`Right`                          | 必须                  |
| `BoolOp`                                             | `Op`、`Values []Expr`                          | 必须                  |
| `Compare`                                            | `Left`、`Ops []string`、`Comparators []Expr`   | 必须                  |
| `Attribute`                                          | `Value`、`Attr`                                | 必须                  |
| `Subscript`                                          | `Value`、`Slice`                               | 必须                  |
| `Slice`                                              | `Lower`、`Upper`、`Step`                       | 必须                  |
| `Call`                                               | `Func`、`Args`、`Keywords []*Keyword`          | 必须                  |
| `Keyword`                                            | `Arg string`（空表示 `**`）、`Value`           | 必须                  |
| `List` / `Tuple` / `Set` / `Dict`                    | `Elts` / `Keys+Values`                         | 必须                  |
| `Lambda`                                             | `Args *Arguments`、`Body Expr`                 | 必须                  |
| `IfExp`                                              | `Test`、`Body`、`OrElse`                       | 必须                  |
| `NamedExpr`                                          | `Target`、`Value`                              | 建议                  |
| `Starred`                                            | `Value`                                        | 建议（调用/赋值解包） |
| `ListComp` / `SetComp` / `DictComp` / `GeneratorExp` | `Elt`/`Key`/`Value`、`Gens`                    | 必须                  |
| `Yield` / `YieldFrom`                                | `Value Expr`（`Yield` 可为 nil）               | 必须（生成器）        |
| `Await`                                              | —                                              | 禁止（致命报错）      |
| `JoinedStr` / `FormattedValue`                       | `Values` / `Value`、`Conversion`、`FormatSpec` | 必须（f-string）      |
| `TemplateStr` / `Interpolation`                      | —                                              | 暂缓                  |

---

## 5. 计划架构

建议目录结构（实现阶段落地）：

```text
.
├── main.go                 # CLI：读取脚本、调用解释器、打印错误、设置退出码
├── module/                 # 标准库（Go 实现），package module，靠 init() 注册
│   ├── registry.go         # 注册表：名字 -> 模块工厂
│   ├── this.go             # import this
│   ├── os.go               # import os
│   ├── sys.go              # import sys
│   └── pathlib.go          # import pathlib
├── gpython_modules/        # 项目依赖：第三方 .py 文件，无需重新编译（已 gitignore）
├── internal/
│   ├── object/             # 运行时对象：None/bool/int/float/str/list/tuple/dict/set
│   │   ├── object.go       # Object 接口、类型名、repr/str、真值判断
│   │   ├── number.go       # 算术与比较
│   │   ├── sequence.go     # list/tuple/str 下标、切片、迭代
│   │   ├── mapping.go      # dict/set
│   │   ├── function.go     # Function、BoundMethod、Builtin、Generator
│   │   ├── class.go        # Class、Instance、MRO、属性查找
│   │   └── exception.go    # Exception 层级与实例
│   ├── interp/
│   │   ├── interp.go       # Interpreter 主体、模块执行
│   │   ├── stmt.go         # 语句求值
│   │   ├── expr.go         # 表达式求值
│   │   ├── env.go          # 作用域链、global/nonlocal
│   │   ├── call.go         # 参数绑定与函数调用
│   │   ├── import.go       # 三级导入解析
│   │   └── control.go      # return/yield/break/continue/异常传播
│   └── builtins/           # print/len/range/next/... 与异常类注册
└── testdata/               # 测试脚本（注意 .gitignore 的 *.py 规则）
```

标准库通过 `main.go` 的 `import _ "gpython/module"` 全量编译并触发各文件的 `init()`，由 `module` 包内的注册表统一登记。

### 5.1 执行管线

```text
读取源文件
  -> parser2.ParseFile(filename, src)
  -> 创建 module 对象与全局作用域
  -> EvalStmts(module.Body)
  -> 正常结束：退出码 0
  -> 解释器异常：格式化 traceback，退出码 1
  -> 语法错误：输出 filename:line:col: 小写消息，退出码 1
```

### 5.2 控制流实现建议

- 返回值：使用内部 `returnSignal{value}`，通过 Go panic 或显式错误返回向上传播。
- `break` / `continue`：内部信号。
- 异常：运行时对象 `*Exception` + 内部 `raiseSignal{exc}`。
- 推荐统一用 `panic` 传播控制流，在函数/循环/`try` 边界 `recover`，这样语句求值函数可以只返回 `error` 或直接不返回错误。需要注意在 `finally` 中正确处理 `return`/`break`/`continue` 对异常的覆盖。
- 每个 `Eval` 入口都要把 Go 的 `runtime.Error` 转成小写内部错误，避免把 Go panic 文本泄露给用户。

### 5.3 作用域

- 模块全局作用域 + 内置作用域。
- 函数局部作用域，定义时捕获闭包环境（支持嵌套函数即可，不要求完整 cell 语义）。
- 类体使用独立命名空间，执行完后转成类属性。
- 查找顺序：局部 -> 闭包 -> 全局 -> 内置（LEGB）。
- `global`、`nonlocal` 按语句声明修改对应作用域。
- 未定义名字报：`nameerror: name 'x' is not defined`。

---

## 6. 四类核心特性的语义设计

### 6.1 `def` 函数

必须支持：

- 普通位置参数、默认参数、`*args`、仅关键字参数、`**kwargs`、仅位置参数 `/`。
- 参数绑定顺序：位置参数 -> 关键字参数 -> `*args` -> 仅关键字 -> `**kwargs`；重复赋值、缺参、未知关键字都要报小写 `typeerror`。
- `return`，无返回值时返回 `None`。
- `yield` / `yield from`：含 `yield` 的函数是生成器函数，调用时返回生成器对象而不立即执行函数体。
  - 生成器实现迭代协议：`__iter__` 返回自身，`__next__` 运行到下一个 `yield`。
  - 迭代结束后抛 `StopIteration`；支持 `for` 循环、`list()`、`next()`。
  - `yield from` 委托另一个可迭代对象。
  - 第一版暂不支持 `send()`、`throw()`、`close()` 和 async 生成器。
- 递归调用、函数作为值传递、嵌套函数与闭包。
- 装饰器按从下到上的顺序应用。
- 方法调用时自动绑定 `self`。
- 函数对象至少保存：名称、参数定义、默认值、函数体、闭包环境、所属模块。
- 暂不支持：async 函数（见 6.5，遇到即致命报错）、完整 `__call__` 协议。

错误示例（全部小写）：

```text
typeerror: f() missing 1 required positional argument: 'x'
typeerror: f() got an unexpected keyword argument 'y'
typeerror: f() takes 2 positional arguments but 3 were given
```

### 6.2 `class` 类

必须支持：

- `class Name:`、`class Name(Base):`、多基类（基础 MRO）。
- 类体中的赋值成为类属性；类体中的 `def` 成为方法。
- 实例化：创建实例，调用 `__init__`，返回实例。
- 属性查找：实例属性 -> 类属性 -> 基类（按 MRO）。
- 方法绑定：通过实例访问函数属性时生成 bound method。
- 继承与 `super()` 的基础用法。
- 类装饰器。
- 用继承内置 `Exception` 的方式定义自定义异常。
- 类属性、实例属性、`__name__`、`__init__`。
- 属性不存在报：`attributeerror: 'int' object has no attribute 'x'`。

暂缓或明确报错：

- 元类（`metaclass=` 关键字可忽略或报 `notimplementederror`）。
- 描述符、`property`、`classmethod`、`staticmethod` 可作为后续增强。
- `__slots__`、`__getattr__`、运算符重载等高级协议。
- 复杂 C3 线性化先实现简化版：深度优先 + 去重，或实现标准 C3；至少保证单继承正确。

### 6.3 `import` 导入

必须支持：

- `import module`
- `import module as name`
- `import a.b`（绑定顶层 `a`，并加载 `a.b`）
- `from module import name`
- `from module import name as alias`
- `from module import *`（可限制为模块没有 `__all__` 时导入非下划线名字）
- 相对导入 `from . import x`、`from ..pkg import y`（基础支持）
- 模块缓存（类似 `sys.modules`），避免重复执行；循环导入返回部分初始化的模块。

模块查找分为三个层级，按顺序命中即返回：

1. **标准库**：`gpython/module` 包。`module/` 目录下每个 Go 文件对应一个标准库模块，文件内 `func init()` 把模块登记进 `module` 包的注册表；`main.go` 用 `import _ "gpython/module"` 触发全量编译与注册。起始标准库：`this`、`os`、`sys`、`pathlib`。
2. **项目依赖**：`gpython_modules/` 目录下的 `.py` 文件。直接读取源码执行，不需要重新编译解释器；该目录已在 `.gitignore` 中忽略。
3. **项目本身**：与 CPython 相同的本地导入机制，查找当前脚本所在目录、当前工作目录（以及可选的 `GPYTHONPATH`）。

示意：

```text
import foo
  -> module 注册表里找 "foo"        （标准库，Go）
  -> 找 gpython_modules/foo.py      （项目依赖，.py）
  -> 找 ./foo.py 或 ./foo/__init__.py（项目本身，.py）
```

文件模块规则：

- `foo.py` 作为模块 `foo`。
- 包 `pkg/__init__.py` 作为模块 `pkg`（基础支持，第一版可先只做单文件模块）。
- 模块执行时的全局作用域即模块的 `__dict__`。
- 同名时按上述层级优先级，标准库优先。

错误示例：

```text
importerror: no module named 'foo'
importerror: cannot import name 'bar' from 'foo'
```

### 6.4 `try/except` 错误处理

必须支持：

- `try` / `except` / `except as name` / 多个 `except` / 裸 `except` / `except (A, B)`。
- `else`：`try` 块无异常时执行。
- `finally`：无论是否异常都执行。
- `raise ValueError("bad")`、`raise ValueError`、裸 `raise` 重新抛出当前异常。
- `raise ... from ...` 保存 cause（至少不丢失）。
- 自定义异常类（继承 `Exception`）。
- 异常匹配按 `except` 顺序进行，匹配规则为 `isinstance`。
- 未捕获异常向上传播，最外层打印 traceback 并以退出码 1 结束。
- `finally` 与 `return`/`break`/`continue` 的组合：`finally` 正常结束后继续原控制流；`finally` 中新的 `return`/`raise` 覆盖原控制流。
- `except ... as e` 结束后可选地删除 `e`（贴近 CPython；不删除也不影响核心功能）。

错误示例：

```text
traceback (most recent call last):
  file "demo.py", line 3, in <module>
  file "demo.py", line 1, in f
zerodivisionerror: division by zero
```

### 6.5 async/await：明确不实现（致命错误）

项目**明确声明不实现** `async` / `await`：

- 并发由 Go 的 goroutine 承担，多线程底层也完全基于 goroutine，因此不引入 Python 的 async/await 模型。
- 只要代码中解析到 `async` 或 `await`，就立即报错并退出（退出码 1）。
- 该错误**不可被 `try/except` 捕获**，也不进入 Python 异常体系；它是解释器的致命诊断。
- 检测发生在执行前的静态扫描阶段，扫描整个模块 AST（以及之后导入的 `.py` 模块）；命中即终止，不会执行任何语句。
- 需要识别的 AST 节点：`AsyncFunctionDef`、`AsyncFor`、`AsyncWith`、`Await`，以及推导式中 `Comprehension.IsAsync == true`。
- 输出格式为全小写，例如：

```text
demo.py:12:1: notimplementederror: async/await is not supported
```

实现要点：在 `internal/interp` 中提供 `checkNoAsync(*parser2.Module) error` 之类的预检函数，返回专用的 fatal 错误；`main.go` 和模块加载路径在求值前调用它，遇到 fatal 错误直接打印并 `os.Exit(1)`，绕过所有 handler。

### 6.6 字符串格式化

必须实现三类语法：

**f-string（`f""` / `rf""` / `fr""`）**

- AST 是 `JoinedStr`，`Values` 中 `Constant` 是普通文本，`FormattedValue` 是插值。
- `FormattedValue.Value` 求值后按 `Conversion` 转换：`-1` 原样、`114` 用 `repr`、`115` 用 `str`、`97` 用 `ascii`（`!a` 第一版可退化为 `repr`）。
- `FormatSpec` 非 nil 时按格式说明符格式化，语法尽量覆盖 `[[fill]align][sign][#][0][width][,][.precision][type]`：
  - 对齐 `<`、`>`、`^`、`=`，可带填充字符；
  - 符号 `+`、`-`、空格；
  - 宽度、`0` 补零、千分位 `,`；
  - 精度 `.N`；
  - 类型 `s`、`d`、`f`、`e`、`g`、`x`、`X`、`o`、`b`、`%`。
- 嵌套 `{}`、转义 `{{`/`}}` 由解析器处理成普通文本，解释器直接使用。
- 不支持的格式说明符报小写 `valueerror`，例如 `valueerror: unknown format code 'q' for object of type 'str'`。

**原始字符串（`r""` / `R""`）**

- 解析器已经把反斜杠原样保留在 `Constant.Value` 中，解释器按普通字符串使用。
- `rf""` / `fr""` 仍是 f-string，只是普通文本片段不做转义。
- 第一版不实现 `b""`、`u""` 前缀的特殊语义（`b""` 仍按 Go string 处理）。

**百分号格式化（`"%..." % value`）**

- AST 是 `BinOp{Op: "Mod"}`，左侧必须是字符串。
- 支持占位符：`%s`、`%r`、`%d`/`%i`、`%f`/`%e`/`%g`、`%x`/`%X`、`%o`、`%%`，以及宽度/精度/`-`/`+`/`0`/空格标志。
- 右侧规则：
  - 单个值：直接替换一个占位符；
  - 元组：按位置依次替换；
  - 字典 + `%(name)s` 形式：按 key 取。
- 错误示例（全部小写）：

```text
typeerror: not all arguments converted during string formatting
typeerror: not enough arguments for format string
valueerror: unsupported format character 'y' at index 3
```

实现建议：在 `internal/object` 放一个 `FormatString` 辅助模块，f-string 的 `FormatSpec` 与 `%` 格式化共用同一套数字/对齐渲染逻辑。

### 6.7 lambda 与三元运算符

**lambda 匿名函数**

- AST 是 `Lambda{Args *Arguments, Body Expr}`，`Args` 与 `def` 共用同一套参数结构（默认值、`*args`、仅关键字、`**kwargs` 都适用）。
- 求值 `lambda` 得到函数对象，调用规则与 `def` 完全一致；函数体是单个表达式，结果即返回值。
- 不支持语句、`yield`、注解；`lambda` 体内不能再写 `lambda` 之外的语句。
- 常见用法：`square = lambda x: x * x`、`add = lambda a, b=1: a + b`。
- 错误示例（小写）：`typeerror: <lambda>() missing 1 required positional argument: 'x'`。

**三元运算符（条件表达式）**

- AST 是 `IfExp{Test, Body, OrElse}`，对应 `Body if Test else OrElse`。
- 只求值被选中的分支：`Test` 为真取 `Body`，否则取 `OrElse`。
- 可与 `lambda`、f-string 嵌套，例如 `lambda v: "big" if v >= 10 else "small"`。
- 与 `and`/`or` 的短路求值一致，不做额外限制。

### 6.8 推导式与生成器表达式

必须实现四种形式：

- 列表推导式 `[expr for target in iter if cond ...]` → `ListComp`
- 集合推导式 `{expr for ...}` → `SetComp`
- 字典推导式 `{key: value for ...}` → `DictComp`
- 生成器表达式 `(expr for ...)` → `GeneratorExp`

语义：

- `Comprehension` 支持多个 `for` 子句和多个 `if` 过滤，按源码顺序嵌套展开。
- 第一个 `for` 的 `Iter` 在外层作用域求值；其余在推导式自己的作用域内求值。
- 推导式有独立作用域，循环变量不泄漏到外层（贴近 CPython）。
- 生成器表达式是惰性的，复用 `yield` 的 `Generator` 对象，逐个产出而不一次性构造列表。
- 支持嵌套推导式；`DictComp` 的键重复时后写覆盖前写。
- `Comprehension.IsAsync == true`（`async for`）属于 async 范畴，按 6.5 致命报错。
- 错误示例（小写）：`nameerror: name 'value' is not defined`（试图在推导式外使用循环变量）。

实现建议：在 `internal/interp` 增加 `evalComprehension`，把 `Gens` 递归展开成嵌套循环；`GeneratorExp` 则包装成惰性生成器。

### 6.9 异常层级（第一版）

```text
baseexception
├── exception
│   ├── arithmeticerror
│   │   ├── zerodivisionerror
│   │   └── overflowerror
│   ├── assertionerror
│   ├── attributeerror
│   ├── importerror
│   │   └── modulenotfounderror
│   ├── indexerror
│   ├── keyerror
│   ├── nameerror
│   ├── notimplementederror
│   ├── runtimeerror
│   ├── typeerror
│   ├── valueerror
│   └── ...
├── keyboardinterrupt（可暂缓）
└── systemexit（可暂缓）
```

内置异常类需要可被 `except` 匹配，也能被用户继承。异常对象保存 `args`、消息、可选的 cause 和 traceback 帧列表。

---

## 7. 错误信息规范（全小写）

适用范围：解释器自己生成的所有诊断文本，包括语法错误、运行时错误、用法错误、traceback 中的异常类型名。

统一格式：

```text
# 语法错误
<file>:<line>:<col>: <lowercase message>

# 运行时错误（可被 try/except 捕获）
traceback (most recent call last):
  file "<file>", line <n>, in <function>
<lowercase-exception-name>: <lowercase message>

# 致命错误（不可捕获，直接退出）
<file>:<line>:<col>: <lowercase message>
```

约定：

- 异常类名在输出时转为小写，例如 `NameError` 输出为 `nameerror`。
- 固定短语全部小写，例如 `name`、`not defined`、`no module named`、`unsupported operand type(s) for`。
- 用户自己传入的异常消息（`raise ValueError("Bad Thing")`）保持原样，不强制改写；只有解释器生成的文案小写。
- `file` 路径原样保留（路径中的大小写属于用户环境，不是错误文案）。
- 不要在错误信息末尾加大写缩写或 Go 的 panic 文本。
- 退出码：成功 `0`，语法/运行时错误/致命错误 `1`，命令行用法错误 `2`。
- `async`/`await` 属于致命错误：不打印可捕获的 traceback，不进入 `try/except`，直接退出。

计划使用的错误文案示例：

```text
demo.py:2:5: expected ':'
nameerror: name 'x' is not defined
typeerror: unsupported operand type(s) for +: 'int' and 'str'
typeerror: object is not callable
attributeerror: 'int' object has no attribute 'x'
indexerror: list index out of range
keyerror: 'k'
importerror: no module named 'foo'
importerror: cannot import name 'bar' from 'foo'
zerodivisionerror: division by zero
valueerror: invalid literal for int(): 'x'
valueerror: unknown format code 'q' for object of type 'str'
demo.py:12:1: notimplementederror: async/await is not supported
```

---

## 8. 命令行与启动方式

计划：

```bash
go run .                 # 按约定运行默认脚本（见下方待确认项）
go run . demo.py         # 运行指定脚本
go run . demo.py arg1    # 脚本参数进入 sys.argv
```

- `main.go` 解析参数，读取文件，调用解释器。
- 脚本执行期间 `sys.argv` 为 `["<script>", "arg1", ...]`。
- 文件不存在：输出小写错误并以退出码 1 结束。
- 顶层未捕获异常：打印 traceback，退出码 1。
- 不支持在启动时指定多个脚本。
- 可选的交互式 REPL（逐行解析 `ParseExpression` 或累积 `ParseFile`）留作后续增强。

### 待确认项

`go run .` 不带参数时的行为需要下一步确认，候选方案：

1. 默认运行仓库根目录的 `demo.py`（最终验收演示脚本）。
2. 进入简单 REPL。
3. 打印用法并以退出码 2 结束。

建议优先方案 1，若 `demo.py` 不存在则打印用法。

---

## 9. 测试与验收计划

`demo.py` 是最终验收演示脚本，覆盖 import / def / class / try-except / 控制流 / 内置函数，并在文件末尾注释中固化期望输出。实现完成后 `go run . demo.py` 应输出该结果并以 0 退出。

测试目录：

- `testdata/` 下放 `.py` 脚本与期望输出。
- `.gitignore` 忽略 `*.py`，仅对 `!demo.py` 放行；`testdata/` 下的脚本需另加例外或改用 `.txt`。
- Go 单元测试覆盖：参数绑定、作用域、属性查找、异常匹配、导入缓存。
- 端到端测试：`go run . testdata/xxx.py` 的 stdout/stderr/退出码。

验收脚本至少覆盖：

1. 三级导入：标准库（`module/` 下的 `this`/`os`/`sys`/`pathlib`）、`gpython_modules/` 下的 `.py` 依赖、脚本目录/工作目录下的本地 `.py`。
2. 带默认参数、`*args`、`**kwargs` 的函数及递归，`return` 与 `yield` 生成器（含 `for` 迭代、`next()`、`StopIteration`）。
3. 单继承、多继承基础、`__init__`、实例属性、方法调用、`super()`。
4. 自定义异常 + `try/except/else/finally` + 裸 `except` + 元组异常 + `raise ... from`。
5. 未捕获异常的小写 traceback 与退出码 1。
6. `async`/`await` 触发不可捕获的致命错误，退出码 1，且 `try/except` 无法拦截。
7. f-string（含 `!r` 与 `:.2f` 格式说明符）、原始字符串 `r"..."`、`%` 格式化（位置、映射、`%%`）。
8. `lambda` 匿名函数（含默认参数）与三元运算符 `a if cond else b`（含与 lambda 嵌套）。
9. 列表/集合/字典推导式（多 `for`、多 `if`）与生成器表达式 `(...)` 的惰性求值。
10. `CGO_ENABLED=0 go build ./...` 成功，且 `module/` 通过 `import _ "gpython/module"` 全量注册。

常用命令：

```bash
go mod tidy
go build ./...
CGO_ENABLED=0 go build ./...
go test ./...
go run .
```

---

## 10. 已知风险与限制

1. `parser2` 只给 AST，所有运行时语义、内置函数、模块系统都要从零实现。
2. 整数是 `int64`，无法表示任意精度整数；大整数脚本会解析失败。
3. 没有源码结束位置和原始文本，错误定位只能到起点 line/col。
4. `except*` 被降级成普通 `except`，不能依赖解析器实现异常组。
5. 解析器不校验上下文（函数外 return 等），解释器必须补充检查。
6. `.gitignore` 忽略所有 `.py`（仅放行 `!demo.py`），`testdata/` 与 `gpython_modules/` 的脚本需额外处理。
7. 完整 Python 语义（描述符、元类、完整 MRO、运算符重载、生成器高级接口）工作量很大，按“limited”原则明确报 `notimplementederror` 比静默错误更好；`yield` 只做基础生成器，不支持 `send`/`throw`/`close`。
8. `async`/`await` 明确不实现：并发交给 Go goroutine，解析到即致命报错且不可捕获；预检必须覆盖整棵 AST 以及后续导入的模块，避免漏网。
9. f-string 与 `%` 格式化只实现常见格式说明符；冷门类型码或复杂嵌套可能报 `valueerror`，`!a` 退化为 `repr`。
10. Go 的 `panic/recover` 控制流需要小心处理 `finally` 覆盖规则，避免吞掉异常。

---

## 11. 信息源

- `/home/PartXI/gpython/README.md`
- `/home/PartXI/gpython/go.mod`
- `/home/PartXI/gpython/.gitignore`
- `github.com/tamnd/gopapy/v2@v2.0.0/doc.go`
- `github.com/tamnd/gopapy/v2@v2.0.0/parser2/ast.go`
- `github.com/tamnd/gopapy/v2@v2.0.0/parser2/parser.go`
- `github.com/tamnd/gopapy/v2@v2.0.0/parser2/stmt.go`
- `github.com/tamnd/gopapy/v2@v2.0.0/parser2/parse_file_test.go`
- 本地探针程序验证：`/tmp/opencode/probe`（仅用于调研，不属于项目代码）
- pkg.go.dev: `github.com/tamnd/gopapy/v2`

---

## 12. 下一步

信息收集与设计已完成并固化在本文件。等待下一步指令后，再按以下顺序实现：

1. 添加依赖并建立运行时对象模型（含 `Generator` 对象）。
2. 实现表达式/语句求值与作用域，支持 `return`/`yield`。
3. 实现字符串：f-string、`r""`、`%` 格式化。
4. 实现 `lambda` 与三元运算符。
5. 实现推导式与生成器表达式。
6. 实现 `async`/`await` 预检：解析到即致命报错、不可捕获。
7. 搭建 `module/` 标准库注册表与 `this`/`os`/`sys`/`pathlib`，在 `main.go` 中 `import _ "gpython/module"`。
8. 实现三级 `import`：标准库 -> `gpython_modules/` -> 项目本地。
9. 实现 `def`、`class`、`try/except`。
10. 接入 `main.go` 与错误输出。
11. 添加测试并验证 `go run .`、`go run . demo.py`、`CGO_ENABLED=0 go build ./...`。
