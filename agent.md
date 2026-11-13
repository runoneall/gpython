# agent.md —— gpython 跨对话工作手册

> 本文件面向“每一次接手本项目的 AI agent / 开发者”。
> 目标：在多次独立对话中，稳定、可交接、可验证地把 gpython 实现完。
> 开工前请依次阅读：本文件 → `TODO.md`（进度与任务）→ `DESIGN.md`（行为依据）。

---

## 1. 项目目标（一句话）

用 **纯 Go（无 cgo）** 实现一个 **简单、受限的 Python 3.14 解释器**，解释执行（遍历 AST，不生成 Go 代码、不编译字节码），入口 `main.go`，用 `go run .` 启动，解析器使用 `github.com/tamnd/gopapy/v2/parser2`。

验收脚本是仓库根目录的 `demo.py`，其文件末尾注释块是**期望输出**。

---

## 2. 不可违反的硬性规则（Non-negotiables）

违反任意一条都算任务失败，改动前先停下来确认。

1. **禁止 cgo。** 任何时刻 `CGO_ENABLED=0 go build ./...` 必须成功。
2. **解析器固定为 `github.com/tamnd/gopapy/v2`（`parser2`）。** 不引入其它解析器，不自己写词法/语法分析。
3. **解释执行。** 不生成 Go 代码，不编译成字节码，直接遍历 `parser2` 的 AST。
4. **入口固定为 `main.go`，启动命令固定为 `go run .`。**
5. **必须实现：** `import`（三级）、`class`、`def`（含 `return`/`yield`）、`try/except`。
6. **必须实现：** `lambda`、三元运算符 `a if cond else b`、f-string `f""`、原始字符串 `r""`、`"%..."` 百分号格式化、列表/集合/字典推导式、生成器表达式 `(...)`。
7. **明确不实现 `async`/`await`。** 源码中只要解析到 `async` 或 `await`，立即报**致命错误**、退出码 1、**不可被 `try/except` 捕获**。并发由 Go goroutine 承担。
8. **解释器自己生成的所有错误信息必须全小写**（异常类型名、固定短语）。用户传入的异常消息（如 `raise ValueError("Bad Thing")`）保持原样。详见 `DESIGN.md` 第 7 节。
9. **不做超出范围的兼容。** 遇到未实现特性时，宁可明确报小写 `notimplementederror`，也不要静默产生错误结果。

---

## 3. 仓库现状（截至 2026-11-13）

```text
/home/PartXI/gpython
├── .gitignore        # *.py / !demo.py / gpython_modules/
├── LICENSE
├── README.md         # 仅 3 行项目描述（M12 再扩写）
├── go.mod            # module gpython / go 1.26.8，require gopapy/v2 v2.0.0
├── go.sum
├── main.go           # CLI：脚本模式 / REPL 模式、参数、退出码、traceback、SystemExit、panic recover
├── DESIGN.md         # 需求与设计依据（source of truth）
├── TODO.md           # M0–M12 分阶段计划与进度（已全部勾选 M0–M12）
├── agent.md          # 本文件
├── demo.py           # 最终验收脚本（.gitignore 例外保留，末尾注释块是期望输出）
├── internal/
│   ├── object/       # 运行时对象：object.go, exception.go, class.go, function.go,
│   │                 # generator.go, sequence.go, number.go, format.go
│   └── interp/       # interp.go, expr.go, stmt.go, call.go, env.go, import.go,
│                     # walk.go, async_check.go, errors.go
└── module/           # 标准库（Go 实现）：registry.go, this.go, os.go, sys.go, pathlib.go, math.go, threading.go, subprocess.go
```

- 解释器已完整实现：tree-walking `parser2` AST，无 cgo，入口 `main.go`，`go run .` 启动。
- M0–M12 全部完成：运行时对象模型、求值核心、函数/闭包/生成器、类/异常/try-except、f-string/r-string/`%`、内置函数与全小写错误体系、async/await 致命预检、标准库注册表（this/os/sys/pathlib/math/threading/subprocess）、三级 import、CLI/traceback/退出码。
- `demo.py` 全绿：`go run . demo.py` 退出码 0，输出与文件末尾期望注释逐行一致。
- 验证：`gofmt -l .` 干净、`go vet ./...` 无告警、`go test ./...` 通过、`CGO_ENABLED=0 go build ./...` 成功。
- 最近提交：`8f78fb5`（新增 `math`/`threading`/`subprocess` 标准库并扩写 `README.md`）；工作区干净，但该提交尚未 push 到 origin/main。

---

## 4. 每次对话的标准工作流

按顺序执行，不要跳步：

1. **读上下文**：`agent.md`（本文件）→ `TODO.md` 顶部进度表与进度日志 → `DESIGN.md` 相关章节。
2. **选里程碑**：从 `TODO.md` 里挑**第一个未完成且依赖已满足**的里程碑（通常按 M0→M12 顺序）。把它的状态改为 `[~] 进行中`。
3. **小步实现**：一次只推进一个里程碑；每个里程碑内按任务清单逐项勾选。改代码时遵循第 6 节约定。
4. **本地验证**：跑该里程碑的“验收”命令，以及 `gofmt`、`go vet`、`go test`、`CGO_ENABLED=0 go build ./...`。
5. **更新文档**：
   - 勾选 `TODO.md` 中已完成的任务；
   - 更新 `TODO.md` 顶部进度表状态；
   - 在 `TODO.md` 末尾“进度日志”追加一条记录（日期 / 里程碑 / 改动文件 / 执行的验证 / 下一步）。
   - 若实现与 `DESIGN.md` 有偏差，改 `DESIGN.md` 并说明原因（`DESIGN.md` 是行为依据）。
6. **交接**：在回复里用几句话说明：完成了哪个里程碑、改了哪些文件、验证结果、下一步建议。

> 一次对话若未完成某个里程碑，保持 `[~] 进行中`，在进度日志里写清“做到哪、卡在哪、下一步”。

---

## 5. 架构地图

```text
.
├── main.go                 # CLI：读脚本、调解释器、打印错误、设置退出码
├── module/                 # 标准库（Go 实现），package module，靠 init() 注册
│   ├── registry.go         # 注册表：名字 -> 模块工厂
│   ├── this.go             # import this
│   ├── os.go               # import os
│   ├── sys.go              # import sys
│   ├── pathlib.go          # import pathlib
│   ├── math.go             # import math（floor/ceil/sqrt 等）
│   ├── threading.go        # import threading（Thread/Lock/RLock/Event）
│   └── subprocess.go       # import subprocess（run/Popen）
├── gpython_modules/        # 项目依赖：第三方 .py，无需重新编译（已 gitignore）
├── internal/
│   ├── object/             # 运行时对象：None/bool/int/float/str/list/tuple/dict/set
│   │   ├── object.go       # Object 接口、类型名、repr/str、真值判断
│   │   ├── number.go       # 算术与比较
│   │   ├── sequence.go     # list/tuple/str 下标、切片、迭代（含 dict/set）
│   │   ├── format.go       # f-string 与 % 格式化共用的渲染逻辑（M5）
│   │   ├── function.go     # Function、BoundMethod、Builtin
│   │   ├── generator.go    # Generator 运行时（M3）
│   │   ├── class.go        # Class、Instance、MRO、属性查找
│   │   └── exception.go    # Exception 层级与实例
│   ├── interp/
│   │   ├── interp.go       # Interpreter 主体、模块执行
│   │   ├── stmt.go         # 语句求值
│   │   ├── expr.go         # 表达式求值
│   │   ├── env.go          # 作用域链、global/nonlocal
│   │   ├── call.go         # 参数绑定与函数调用
│   │   ├── import.go       # 三级导入解析（M9）
│   │   ├── errors.go       # 控制信号、FatalError / SyntaxError / wrapParseError
│   │   ├── async_check.go  # checkNoAsync 预检（M7）
│   └── builtins/           # print/len/range/next/... 与异常类注册
└── testdata/               # 测试脚本（注意 .gitignore 的 *.py 规则，见第 8 节）
```

### 5.1 执行管线

```text
读取源文件
  -> parser2.ParseFile(filename, src)          # 语法错误：file:line:col: <小写消息>，退出 1
  -> checkNoAsync(module)                       # 命中 async/await：致命错误，退出 1，不可捕获
  -> 创建 module 对象与全局作用域
  -> EvalStmts(module.Body)
  -> 正常结束：退出码 0
  -> 解释器异常：格式化小写 traceback，退出码 1
```

### 5.2 控制流

- `return` / `yield` / `break` / `continue` / 异常：用内部信号 + `panic/recover` 传播（在函数、循环、`try` 边界 recover）。
- 关键难点：`finally` 对 `return`/`break`/`continue`/异常的覆盖规则，必须显式处理，避免吞掉异常。
- **`async`/`await` 的致命错误绝不能走 `raiseSignal`**，必须绕过所有 `try/except`。

### 5.3 三级导入顺序

1. `module/` 注册表（Go 标准库：`this`/`os`/`sys`/`pathlib`/`math`）。
2. `gpython_modules/` 下的 `.py`（项目依赖，不重新编译；识别 `rel/__init__.py` 作为包）。
3. 脚本目录 / 工作目录下的本地 `.py`（可支持 `GPYTHONPATH`；识别 `rel/__init__.py` 作为包）。

---

## 6. 编码约定

- **包边界**：`internal/object` 不依赖 `internal/interp`；`module` 只依赖 `internal/object`，避免循环依赖。
- **错误类型**（`internal/interp/errors.go`）：
  - `RuntimeError`：对应 Python 异常，可被 `try/except` 捕获；
  - `FatalError`：不可捕获（`async`/`await`、内部致命问题）；
  - `SyntaxError`：包装 parser2 的 `filename:line:col: message`。
- **错误文案集中管理**：所有解释器生成的文案放在少量文件里，便于统一检查“全小写”。
- **类型名小写输出**：`NameError` → `nameerror`；traceback 中的异常类型名同样小写。
- **退出码**：成功 `0`；语法/运行时/致命错误 `1`；命令行用法错误 `2`。
- **不要泄露 Go panic 文本**：最外层用 `recover` 兜底，转成小写内部错误。
- **AST 字段命名**以 `DESIGN.md` 第 3、4 节为准（`Constant.Kind/Value`、`JoinedStr.Values`、`FormattedValue.Conversion`、`Arguments` 的默认值规则等）。
- **parser2 不做语义检查**：函数外 `return`、循环外 `break`/`continue`、`except*` 等都要解释器自己校验或明确拒绝。
- 新增代码前先看同目录既有文件的风格（命名、注释语言、错误处理方式），保持一致。

---

## 7. 测试与验证

### 7.1 常用命令

```bash
go get github.com/tamnd/gopapy/v2@v2.0.0   # 仅 M0 执行一次
go mod tidy
gofmt -l .
go vet ./...
go test ./...
CGO_ENABLED=0 go build ./...
go run .
go run . demo.py
```

### 7.2 `demo.py` 逐行校验（M11 必做）

期望输出藏在 `demo.py` 末尾注释块中。用以下脚本比对（需要 `python3.13` 作为参考实现）：

```bash
cd /home/PartXI/gpython
python3.13 demo.py > /tmp/opencode/demo_actual.txt 2>/tmp/opencode/demo_actual.err
echo "exit=$? stderr=$(wc -c < /tmp/opencode/demo_actual.err)"
python3.13 - <<'EOF'
from pathlib import Path
import difflib
lines = Path('demo.py').read_text().splitlines()
start = next(i for i, l in enumerate(lines) if l.startswith('# Expected output (go run'))
end = next(i for i in range(start + 1, len(lines)) if lines[i].startswith('# ---'))
expected = []
for l in lines[start + 1:end]:
    if l.startswith('# '):
        expected.append(l[2:])
    elif l.startswith('#'):
        expected.append(l[1:])
actual = Path('/tmp/opencode/demo_actual.txt').read_text().splitlines()
print('expected output matches actual output' if expected == actual else '\n'.join(difflib.unified_diff(expected, actual, 'expected', 'actual', lineterm='')))
EOF
```

gpython 实现完成后，把上面第一行换成 `go run . demo.py`，期望同样输出 `expected output matches actual output`，退出码 0，stderr 为空。

### 7.3 测试脚本放置约定（`.gitignore` 坑）

`.gitignore` 忽略所有 `*.py`，仅放行 `!demo.py`。因此 `testdata/` 下的测试脚本有两种处理方式，任选其一并在 `TODO.md` 记录：

- 在 `.gitignore` 增加 `!testdata/*.py`、`!gpython_modules/**` 等例外；或
- 测试脚本用 `.txt` 后缀存放，测试时再改名/读取。

---

## 8. 关键行为速查（摘自 `DESIGN.md`，以 `DESIGN.md` 为准）

- 常量：`int`(int64)、`float`(float64)、`str`、`True`/`False`/`None`/`Ellipsis`；`complex`/`bytes` 第一版可明确不支持。
- `r"..."`：解析器已保留反斜杠，按普通字符串用。
- f-string：`JoinedStr.Values` 中 `Constant` 是文本、`FormattedValue` 是插值；`Conversion` `-1`/`114`(`!r`)/`115`(`!s`)/`97`(`!a`)；`FormatSpec` 也是 `JoinedStr`。
- `%` 格式化：`BinOp{Op:"Mod"}`，左侧字符串；右侧单值 / 元组 / `%(name)s` 映射。
- 推导式：独立作用域，循环变量不泄漏；`GeneratorExp` 惰性；`Comprehension.IsAsync` 属 async 致命错误。
- 异常层级见 `DESIGN.md` 6.9；内置异常类可被 `except` 匹配，也可被用户继承。

---

## 9. 交接协议（Handoff Protocol）

每次对话结束，保证满足：

1. 代码可构建：`CGO_ENABLED=0 go build ./...` 成功。
2. 测试可跑：`go test ./...` 通过（若该阶段还没有测试，则至少 `go vet ./...` 干净）。
3. `TODO.md` 已更新：任务勾选 + 进度表 + 进度日志。
4. 若行为有变化，`DESIGN.md` 已同步。
5. 回复中给出：**完成内容 / 改动文件 / 验证结果 / 下一步**。

### 进度日志模板

```markdown
- YYYY-MM-DD：完成 Mx（一句话）；改动：<文件列表>；验证：<命令与结果>；下一步：<My 或待办>。
```

---

## 10. 未决问题（实现时确认后再定稿）

> 下列 5 项已在 2026-11-13 会话中统一决策并落地代码，无需再讨论：

1. `go run .` 无参数时：**进入 REPL**（`>>> ` / `... ` 提示、EOF 退出、表达式打印 `repr`、多语句共享全局 env）；`demo.py` 需显式指定 `go run . demo.py` 运行。（已落地 `main.go`）
2. `gpython_modules/` **需要 `__init__.py` 包支持**：`internal/interp/import.go` `findModuleFile` 已对 `deps`（`gpython_modules/`）与 `locals` 统一检查 `rel/__init__.py`，无需额外改动。
3. `this`：**打印 Zen of Python**，行为与 CPython 标准库一致，import 时自动打印 ROT13 内容。（已落地 `module/this.go`，`demo.py` 已通过）
4. 标准库：`math`（`module/math.go`，floor/ceil/sqrt 等）、`threading`（`module/threading.go`）、`subprocess`（`module/subprocess.go`）均已落地，`demo.py` 第 1 节全部通过。（已落地，见提交 8f78fb5）
5. `module/` 模块工厂与 `internal/object` 的接口边界：已定稿，`module` 只依赖 `internal/object`，避免循环依赖。
