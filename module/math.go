package module

import (
	"math"

	"gpython/internal/object"
)

func init() {
	Register("math", newMathModule)
}

func newMathModule() *object.Module {
	m := object.NewModule("math")
	m.Dict["pi"] = object.NewFloat(math.Pi)
	m.Dict["e"] = object.NewFloat(math.E)
	m.Dict["tau"] = object.NewFloat(2 * math.Pi)
	m.Dict["inf"] = object.NewFloat(math.Inf(1))
	m.Dict["nan"] = object.NewFloat(math.NaN())

	m.Dict["floor"] = object.NewBuiltin("floor", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("floor", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.NewInt(int64(math.Floor(f))), nil
	})
	m.Dict["ceil"] = object.NewBuiltin("ceil", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("ceil", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.NewInt(int64(math.Ceil(f))), nil
	})
	m.Dict["trunc"] = object.NewBuiltin("trunc", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("trunc", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.NewInt(int64(math.Trunc(f))), nil
	})
	m.Dict["sqrt"] = object.NewBuiltin("sqrt", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("sqrt", args, kwargs)
		if err != nil {
			return nil, err
		}
		if f < 0 {
			return nil, object.Raisef(object.ValueErrorClass, "math domain error")
		}
		return object.NewFloat(math.Sqrt(f)), nil
	})
	m.Dict["fabs"] = object.NewBuiltin("fabs", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("fabs", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.NewFloat(math.Abs(f)), nil
	})
	m.Dict["pow"] = object.NewBuiltin("pow", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 2 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "pow() takes exactly 2 arguments")
		}
		x, ok1 := numericFloat(args[0])
		y, ok2 := numericFloat(args[1])
		if !ok1 || !ok2 {
			return nil, object.Raisef(object.TypeErrorClass, "pow() arguments must be numbers")
		}
		return object.NewFloat(math.Pow(x, y)), nil
	})
	m.Dict["exp"] = object.NewBuiltin("exp", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("exp", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.NewFloat(math.Exp(f)), nil
	})
	m.Dict["log"] = object.NewBuiltin("log", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) < 1 || len(args) > 2 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "log() takes 1 or 2 arguments")
		}
		f, ok := numericFloat(args[0])
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "log() argument must be a number")
		}
		if f <= 0 {
			return nil, object.Raisef(object.ValueErrorClass, "math domain error")
		}
		if len(args) == 2 {
			b, ok := numericFloat(args[1])
			if !ok {
				return nil, object.Raisef(object.TypeErrorClass, "log() base must be a number")
			}
			return object.NewFloat(math.Log(f) / math.Log(b)), nil
		}
		return object.NewFloat(math.Log(f)), nil
	})
	m.Dict["factorial"] = object.NewBuiltin("factorial", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) != 1 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "factorial() takes exactly 1 argument")
		}
		n, ok := intValue(args[0])
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "factorial() only accepts integral values")
		}
		if n < 0 {
			return nil, object.Raisef(object.ValueErrorClass, "factorial() not defined for negative values")
		}
		result := int64(1)
		for i := int64(2); i <= n; i++ {
			result *= i
		}
		return object.NewInt(result), nil
	})
	m.Dict["gcd"] = object.NewBuiltin("gcd", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		if len(args) < 2 || len(kwargs) != 0 {
			return nil, object.Raisef(object.TypeErrorClass, "gcd() takes at least 2 arguments")
		}
		a, ok := intValue(args[0])
		if !ok {
			return nil, object.Raisef(object.TypeErrorClass, "gcd() arguments must be integers")
		}
		if a < 0 {
			a = -a
		}
		for _, arg := range args[1:] {
			b, ok := intValue(arg)
			if !ok {
				return nil, object.Raisef(object.TypeErrorClass, "gcd() arguments must be integers")
			}
			if b < 0 {
				b = -b
			}
			for b != 0 {
				a, b = b, a%b
			}
		}
		return object.NewInt(a), nil
	})
	m.Dict["isnan"] = object.NewBuiltin("isnan", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("isnan", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.BoolFrom(math.IsNaN(f)), nil
	})
	m.Dict["isinf"] = object.NewBuiltin("isinf", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("isinf", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.BoolFrom(math.IsInf(f, 0)), nil
	})
	m.Dict["isfinite"] = object.NewBuiltin("isfinite", func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat("isfinite", args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.BoolFrom(!math.IsInf(f, 0) && !math.IsNaN(f)), nil
	})
	m.Dict["sin"] = floatFunc("sin", math.Sin)
	m.Dict["cos"] = floatFunc("cos", math.Cos)
	m.Dict["tan"] = floatFunc("tan", math.Tan)
	m.Dict["radians"] = floatFunc("radians", func(x float64) float64 { return x * math.Pi / 180 })
	m.Dict["degrees"] = floatFunc("degrees", func(x float64) float64 { return x * 180 / math.Pi })
	return m
}

func oneFloat(name string, args []object.Object, kwargs map[string]object.Object) (float64, error) {
	if len(args) != 1 || len(kwargs) != 0 {
		return 0, object.Raisef(object.TypeErrorClass, "%s() takes exactly 1 argument", name)
	}
	f, ok := numericFloat(args[0])
	if !ok {
		return 0, object.Raisef(object.TypeErrorClass, "%s() argument must be a number", name)
	}
	return f, nil
}

func numericFloat(o object.Object) (float64, bool) {
	switch x := o.(type) {
	case *object.Int:
		return float64(x.V), true
	case *object.Float:
		return x.V, true
	case object.Bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func intValue(o object.Object) (int64, bool) {
	switch x := o.(type) {
	case *object.Int:
		return x.V, true
	case object.Bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func floatFunc(name string, fn func(float64) float64) *object.Builtin {
	return object.NewBuiltin(name, func(ctx object.Caller, args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
		f, err := oneFloat(name, args, kwargs)
		if err != nil {
			return nil, err
		}
		return object.NewFloat(fn(f)), nil
	})
}
