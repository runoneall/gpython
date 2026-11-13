package module

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"gpython/internal/object"
)

// CompletedProcess is the result of subprocess.run().
type CompletedProcess struct {
	args       []object.Object
	returncode int
	stdout     string
	stderr     string
}

func NewCompletedProcess(args []object.Object, returncode int, stdout, stderr string) *CompletedProcess {
	return &CompletedProcess{args: args, returncode: returncode, stdout: stdout, stderr: stderr}
}

func (c *CompletedProcess) TypeName() string { return "completedprocess" }
func (c *CompletedProcess) Repr() string {
	return "<CompletedProcess(args=" + argsRepr(c.args) + ", returncode=" + fmt.Sprintf("%d", c.returncode) + ")>"
}
func (c *CompletedProcess) Str() string { return c.Repr() }
func (*CompletedProcess) Truthy() bool  { return true }

func (c *CompletedProcess) GetAttr(name string) (object.Object, error) {
	switch name {
	case "returncode":
		return object.NewInt(int64(c.returncode)), nil
	case "stdout":
		return object.NewStr(c.stdout), nil
	case "stderr":
		return object.NewStr(c.stderr), nil
	case "args":
		return object.NewTuple(c.args...), nil
	}
	return nil, object.Raisef(object.AttributeError, "'CompletedProcess' object has no attribute '%s'", name)
}

func (c *CompletedProcess) SetAttr(name string, value object.Object) error {
	return object.Raisef(object.AttributeError, "'CompletedProcess' object attribute '%s' is read-only", name)
}

// Popen is a subprocess.Popen-compatible value.
type Popen struct {
	args      []object.Object
	cmd       *exec.Cmd
	stdoutBuf bytes.Buffer
	stderrBuf bytes.Buffer
	stdinPipe io.WriteCloser

	started    bool
	finished   bool
	returncode int
	err        error
	done       chan struct{}
	mu         sync.Mutex
}

func NewPopen(args []object.Object) (*Popen, error) {
	cmdStrs, err := argsToStrings(args)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(cmdStrs[0], cmdStrs[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, object.Raisef(object.OSError, "cannot create stdout pipe: %s", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, object.Raisef(object.OSError, "cannot create stderr pipe: %s", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, object.Raisef(object.OSError, "cannot create stdin pipe: %s", err)
	}

	p := &Popen{
		args:      args,
		cmd:       cmd,
		stdinPipe: stdin,
		done:      make(chan struct{}),
	}

	go func() {
		_, _ = io.Copy(&p.stdoutBuf, stdout)
		_, _ = io.Copy(&p.stderrBuf, stderr)
		err := cmd.Wait()
		p.mu.Lock()
		p.finished = true
		p.returncode = cmd.ProcessState.ExitCode()
		p.err = err
		p.mu.Unlock()
		close(p.done)
	}()

	if err := cmd.Start(); err != nil {
		return nil, object.Raisef(object.OSError, "cannot start process: %s", err)
	}
	p.started = true
	return p, nil
}

func (p *Popen) TypeName() string { return "popen" }
func (p *Popen) Repr() string {
	state := "not started"
	if p.started {
		state = "running"
	}
	if p.finished {
		state = "finished"
	}
	return "<Popen(" + argsRepr(p.args) + ", " + state + ")>"
}
func (p *Popen) Str() string { return p.Repr() }
func (*Popen) Truthy() bool  { return true }

func (p *Popen) GetAttr(name string) (object.Object, error) {
	switch name {
	case "returncode":
		p.mu.Lock()
		rc := p.returncode
		p.mu.Unlock()
		return object.NewInt(int64(rc)), nil
	case "stdin":
		return object.NewStr("<stdin pipe>"), nil
	case "stdout":
		return object.NewStr("<stdout pipe>"), nil
	case "stderr":
		return object.NewStr("<stderr pipe>"), nil
	case "poll":
		return object.NewBuiltin("poll", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "poll() takes no arguments")
			}
			p.mu.Lock()
			rc := p.returncode
			finished := p.finished
			p.mu.Unlock()
			if !finished {
				return object.None, nil
			}
			return object.NewInt(int64(rc)), nil
		}), nil
	case "wait":
		return object.NewBuiltin("wait", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) != 0 || len(kwargs) != 0 {
				return nil, object.Raisef(object.TypeErrorClass, "wait() takes no arguments")
			}
			<-p.done
			p.mu.Lock()
			rc := p.returncode
			p.mu.Unlock()
			return object.NewInt(int64(rc)), nil
		}), nil
	case "communicate":
		return object.NewBuiltin("communicate", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
			if len(args) > 1 {
				return nil, object.Raisef(object.TypeErrorClass, "communicate() takes at most 1 argument")
			}
			if len(args) == 1 {
				if _, ok := args[0].(*object.NoneType); !ok {
					// Only None input is supported (no data to send).
					return nil, object.Raisef(object.NotImplementedError, "communicate() input data is not supported")
				}
			}
			<-p.done
			p.mu.Lock()
			stdout := p.stdoutBuf.String()
			stderr := p.stderrBuf.String()
			p.mu.Unlock()
			return object.NewTuple(object.NewStr(stdout), object.NewStr(stderr)), nil
		}), nil
	}
	return nil, object.Raisef(object.AttributeError, "'Popen' object has no attribute '%s'", name)
}

func (p *Popen) SetAttr(name string, value object.Object) error {
	return object.Raisef(object.AttributeError, "'Popen' object attribute '%s' is read-only", name)
}

func newSubprocessModule() *object.Module {
	m := object.NewModule("subprocess")

	m.Dict["run"] = object.NewBuiltin("run", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) < 1 {
			return nil, object.Raisef(object.TypeErrorClass, "run() requires at least 1 argument (args)")
		}
		captureOutput := false
		if v, ok := kwargs["capture_output"]; ok {
			if b, ok := v.(object.Bool); ok {
				captureOutput = bool(b)
			} else {
				return nil, object.Raisef(object.TypeErrorClass, "capture_output must be bool")
			}
		}
		delete(kwargs, "capture_output")
		delete(kwargs, "input")
		delete(kwargs, "timeout")
		if len(kwargs) > 0 {
			for k := range kwargs {
				return nil, object.Raisef(object.TypeErrorClass, "run() got an unexpected keyword argument '%s'", k)
			}
		}

		var cmdElts []object.Object
		switch a := args[0].(type) {
		case *object.Tuple:
			cmdElts = a.Elts
		case *object.List:
			cmdElts = a.Elts
		case *object.Str:
			// A single string is treated as one command with no args.
			cmdElts = []object.Object{a}
		default:
			return nil, object.Raisef(object.TypeErrorClass, "args must be a str, tuple, or list")
		}
		cmdStrs, err := argsToStrings(cmdElts)
		if err != nil {
			return nil, err
		}
		cmd := exec.Command(cmdStrs[0], cmdStrs[1:]...)

		var stdoutBuf, stderrBuf bytes.Buffer
		if captureOutput {
			cmd.Stdout = &stdoutBuf
			cmd.Stderr = &stderrBuf
		}

		err = cmd.Run()
		rc := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				rc = exitErr.ExitCode()
			} else {
				return nil, object.Raisef(object.OSError, "cannot run process: %s", err)
			}
		}

		return NewCompletedProcess(cmdElts, rc, stdoutBuf.String(), stderrBuf.String()), nil
	})

	m.Dict["Popen"] = object.NewBuiltin("Popen", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) < 1 {
			return nil, object.Raisef(object.TypeErrorClass, "Popen() requires at least 1 argument (args)")
		}
		var cmdElts []object.Object
		switch a := args[0].(type) {
		case *object.Tuple:
			cmdElts = a.Elts
		case *object.List:
			cmdElts = a.Elts
		case *object.Str:
			cmdElts = []object.Object{a}
		default:
			return nil, object.Raisef(object.TypeErrorClass, "args must be a str, tuple, or list")
		}
		return NewPopen(cmdElts)
	})

	return m
}

func init() {
	Register("subprocess", newSubprocessModule)
}

func argsToStrings(args []object.Object) ([]string, error) {
	strs := make([]string, len(args))
	for i, a := range args {
		switch x := a.(type) {
		case *object.Str:
			strs[i] = x.V
		case *object.Int:
			strs[i] = fmt.Sprintf("%d", x.V)
		case *object.Float:
			strs[i] = fmt.Sprintf("%g", x.V)
		case object.Bool:
			if bool(x) {
				strs[i] = "True"
			} else {
				strs[i] = "False"
			}
		default:
			return nil, object.Raisef(object.TypeErrorClass, "args elements must be str or number, not '%s'", a.TypeName())
		}
	}
	if len(strs) == 0 {
		return nil, object.Raisef(object.ValueErrorClass, "args is empty")
	}
	return strs, nil
}

func argsRepr(args []object.Object) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.Repr()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
