package object

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type formatSpec struct {
	fill      rune
	align     rune
	sign      rune
	alt       bool
	zero      bool
	width     int
	comma     bool
	precision int
	typ       rune
}

func parseFormatSpec(s string) (*formatSpec, error) {
	fs := &formatSpec{fill: ' ', precision: -1}
	r := []rune(s)
	i := 0
	if len(r) == 0 {
		return fs, nil
	}
	if len(r) >= 2 && isAlign(r[1]) {
		fs.fill = r[0]
		fs.align = r[1]
		i = 2
	} else if isAlign(r[0]) {
		fs.align = r[0]
		i = 1
	}
	if i < len(r) && (r[i] == '+' || r[i] == '-' || r[i] == ' ') {
		fs.sign = r[i]
		i++
	}
	if i < len(r) && r[i] == '#' {
		fs.alt = true
		i++
	}
	if i < len(r) && r[i] == '0' {
		fs.zero = true
		fs.fill = '0'
		fs.align = '='
		i++
	}
	start := i
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		fs.width = fs.width*10 + int(r[i]-'0')
		i++
	}
	if i > start && fs.width == 0 && fs.fill == '0' {
		// "0" consumed as a width digit is harmless.
	}
	if i < len(r) && r[i] == ',' {
		fs.comma = true
		i++
	}
	if i < len(r) && r[i] == '.' {
		i++
		fs.precision = 0
		start = i
		for i < len(r) && r[i] >= '0' && r[i] <= '9' {
			fs.precision = fs.precision*10 + int(r[i]-'0')
			i++
		}
		if i == start {
			return nil, Raisef(ValueErrorClass, "format specifier missing precision")
		}
	}
	if i < len(r) {
		fs.typ = r[i]
		i++
	}
	if i != len(r) {
		return nil, Raisef(ValueErrorClass, "invalid format specifier '%s'", s)
	}
	return fs, nil
}

func isAlign(r rune) bool {
	return r == '<' || r == '>' || r == '^' || r == '='
}

// FormatValue renders v according to a Python format specification.
func FormatValue(v Object, spec string) (string, error) {
	fs, err := parseFormatSpec(spec)
	if err != nil {
		return "", err
	}
	switch x := v.(type) {
	case *Str:
		if fs.typ == 0 || fs.typ == 's' {
			body := x.V
			if fs.precision >= 0 {
				body = truncateRunes(body, fs.precision)
			}
			return padText(body, fs), nil
		}
		return "", unknownCode(fs.typ, v)
	case Bool:
		if fs.typ == 0 {
			return padText(x.Str(), fs), nil
		}
		return formatInt(int64(boolInt(x)), x.Repr(), fs, v)
	case *Int:
		if fs.typ == 0 {
			return padText(x.Repr(), fs), nil
		}
		return formatInt(x.V, x.Repr(), fs, v)
	case *Float:
		if fs.typ == 0 {
			body := x.Repr()
			if fs.precision >= 0 {
				body = strconv.FormatFloat(x.V, 'g', fs.precision, 64)
			}
			return padNumeric("", "", body, fs), nil
		}
		return formatFloat(x.V, fs, v)
	}
	if fs.typ == 0 || fs.typ == 's' {
		body := v.Str()
		if fs.precision >= 0 {
			body = truncateRunes(body, fs.precision)
		}
		return padText(body, fs), nil
	}
	return "", unknownCode(fs.typ, v)
}

func unknownCode(code rune, v Object) error {
	return Raisef(ValueErrorClass, "unknown format code '%c' for object of type '%s'", code, v.TypeName())
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func formatInt(i int64, plain string, fs *formatSpec, v Object) (string, error) {
	switch fs.typ {
	case 'd', 'n':
		return formatInteger(i, 10, "", false, fs), nil
	case 'x':
		return formatBase(i, 16, false, fs), nil
	case 'X':
		return formatBase(i, 16, true, fs), nil
	case 'o':
		return formatBase(i, 8, false, fs), nil
	case 'b':
		return formatBase(i, 2, false, fs), nil
	case 'c':
		return padText(string(rune(i)), fs), nil
	case 'f', 'F', 'e', 'E', 'g', 'G', '%':
		return formatFloat(float64(i), fs, v)
	}
	_ = plain
	return "", unknownCode(fs.typ, v)
}

func formatBase(i int64, base int, upper bool, fs *formatSpec) string {
	neg := i < 0
	var mag uint64
	if neg {
		mag = uint64(-(i + 1)) + 1
	} else {
		mag = uint64(i)
	}
	body := strconv.FormatUint(mag, base)
	if upper {
		body = strings.ToUpper(body)
	}
	prefix := ""
	if fs.alt {
		switch base {
		case 16:
			if upper {
				prefix = "0X"
			} else {
				prefix = "0x"
			}
		case 8:
			prefix = "0o"
		case 2:
			prefix = "0b"
		}
	}
	if fs.precision > len(body) {
		body = strings.Repeat("0", fs.precision-len(body)) + body
	}
	sign := signString(neg, fs.sign)
	return padNumeric(sign, prefix, body, fs)
}

func formatInteger(i int64, base int, prefix string, upper bool, fs *formatSpec) string {
	neg := i < 0
	var mag uint64
	if neg {
		mag = uint64(-(i + 1)) + 1
	} else {
		mag = uint64(i)
	}
	body := strconv.FormatUint(mag, base)
	if fs.comma {
		body = groupDigits(body)
	}
	if fs.precision > len(body) {
		body = strings.Repeat("0", fs.precision-len(body)) + body
	}
	sign := signString(neg, fs.sign)
	return padNumeric(sign, prefix, body, fs)
}

func groupDigits(s string) string {
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		if len(s) > pre {
			b.WriteByte(',')
		}
	}
	for i := pre; i < len(s); i += 3 {
		if i > pre {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func formatFloat(f float64, fs *formatSpec, v Object) (string, error) {
	typ := fs.typ
	prec := fs.precision
	if typ == 0 {
		typ = 'g'
		if prec < 0 {
			prec = -1
		}
	}
	neg := math.Signbit(f) && !math.IsNaN(f)
	af := math.Abs(f)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		body := "nan"
		if math.IsInf(f, 1) {
			body = "inf"
		} else if math.IsInf(f, -1) {
			body = "-inf"
			neg = false
		}
		sign := signString(neg, fs.sign)
		return padNumeric(sign, "", body, fs), nil
	}
	var body string
	switch typ {
	case 'f', 'F':
		if prec < 0 {
			prec = 6
		}
		body = strconv.FormatFloat(af, 'f', prec, 64)
		if typ == 'F' {
			body = strings.ToUpper(body)
		}
	case 'e', 'E':
		if prec < 0 {
			prec = 6
		}
		body = strconv.FormatFloat(af, 'e', prec, 64)
		if typ == 'E' {
			body = strings.ToUpper(body)
		}
	case 'g', 'G':
		body = strconv.FormatFloat(af, 'g', prec, 64)
		if typ == 'G' {
			body = strings.ToUpper(body)
		}
	case '%':
		if prec < 0 {
			prec = 6
		}
		body = strconv.FormatFloat(af*100, 'f', prec, 64) + "%"
	default:
		return "", unknownCode(typ, v)
	}
	if fs.comma && (typ == 'f' || typ == 'F') {
		dot := strings.IndexByte(body, '.')
		if dot < 0 {
			dot = len(body)
		}
		body = groupDigits(body[:dot]) + body[dot:]
	}
	sign := signString(neg, fs.sign)
	return padNumeric(sign, "", body, fs), nil
}

func signString(neg bool, sign rune) string {
	if neg {
		return "-"
	}
	switch sign {
	case '+':
		return "+"
	case ' ':
		return " "
	}
	return ""
}

func padNumeric(sign, prefix, body string, fs *formatSpec) string {
	if fs.precision > 0 && sign == "" && prefix == "" {
		// precision already applied by callers
	}
	align := fs.align
	if align == 0 {
		align = '>'
	}
	fill := fs.fill
	if fill == 0 {
		fill = ' '
	}
	total := len(sign) + len(prefix) + len(body)
	if fs.width <= total {
		return sign + prefix + body
	}
	n := fs.width - total
	pad := strings.Repeat(string(fill), n)
	switch align {
	case '<':
		return sign + prefix + body + pad
	case '^':
		left := n / 2
		right := n - left
		return strings.Repeat(string(fill), left) + sign + prefix + body + strings.Repeat(string(fill), right)
	case '=':
		return sign + prefix + pad + body
	default:
		return pad + sign + prefix + body
	}
}

func padText(body string, fs *formatSpec) string {
	align := fs.align
	if align == 0 || align == '=' {
		align = '<'
	}
	fill := fs.fill
	if fill == 0 || fill == '0' {
		fill = ' '
	}
	n := utf8.RuneCountInString(body)
	if fs.width <= n {
		return body
	}
	pad := strings.Repeat(string(fill), fs.width-n)
	switch align {
	case '>':
		return pad + body
	case '^':
		left := (fs.width - n) / 2
		right := fs.width - n - left
		return strings.Repeat(string(fill), left) + body + strings.Repeat(string(fill), right)
	default:
		return body + pad
	}
}

// PercentFormat implements `"..." % value`.
func PercentFormat(format string, arg Object) (Object, error) {
	args, isTuple := []Object{arg}, false
	if t, ok := arg.(*Tuple); ok {
		args = t.Elts
		isTuple = true
	}
	var out strings.Builder
	used := 0
	mapping := false

	for i := 0; i < len(format); {
		c := format[i]
		if c != '%' {
			out.WriteByte(c)
			i++
			continue
		}
		if i+1 < len(format) && format[i+1] == '%' {
			out.WriteByte('%')
			i += 2
			continue
		}
		i++ // skip '%'
		key := ""
		isKey := false
		if i < len(format) && format[i] == '(' {
			end := strings.IndexByte(format[i:], ')')
			if end < 0 {
				return nil, Raisef(ValueErrorClass, "incomplete format key")
			}
			key = format[i+1 : i+end]
			isKey = true
			mapping = true
			i += end + 1
		}
		var flags strings.Builder
		for i < len(format) && strings.ContainsRune("-+ #0", rune(format[i])) {
			flags.WriteByte(format[i])
			i++
		}
		width := ""
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			width += string(format[i])
			i++
		}
		prec := ""
		if i < len(format) && format[i] == '.' {
			i++
			prec = "."
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				prec += string(format[i])
				i++
			}
			if prec == "." {
				prec = ".0"
			}
		}
		if i >= len(format) {
			return nil, Raisef(ValueErrorClass, "incomplete format")
		}
		conv := format[i]
		convIndex := i
		i++

		var value Object
		if isKey {
			dict, ok := arg.(*Dict)
			if !ok {
				return nil, Raisef(TypeErrorClass, "format requires a mapping")
			}
			v, found := dict.Get(NewStr(key))
			if !found {
				return nil, Raise(KeyError, NewStr(key))
			}
			value = v
		} else {
			if used >= len(args) {
				return nil, Raisef(TypeErrorClass, "not enough arguments for format string")
			}
			value = args[used]
			used++
		}
		rendered, err := renderPercent(value, flags.String(), width, prec, conv, convIndex)
		if err != nil {
			return nil, err
		}
		out.WriteString(rendered)
	}

	if !mapping && used < len(args) {
		if !isTuple || len(args) > 0 {
			return nil, Raisef(TypeErrorClass, "not all arguments converted during string formatting")
		}
	}
	return NewStr(out.String()), nil
}

func renderPercent(v Object, flags, width, prec string, conv byte, index int) (string, error) {
	specFlags := flags
	spec := ""
	if strings.Contains(specFlags, "-") {
		spec += "<"
	} else {
		spec += ">"
	}
	if strings.Contains(specFlags, "+") {
		spec += "+"
	} else if strings.Contains(specFlags, " ") {
		spec += " "
	}
	if strings.Contains(specFlags, "#") {
		spec += "#"
	}
	if strings.Contains(specFlags, "0") && !strings.Contains(specFlags, "-") {
		spec += "0"
	}
	spec += width
	spec += prec

	switch conv {
	case 's':
		s := v.Str()
		if prec != "" {
			n, _ := strconv.Atoi(strings.TrimPrefix(prec, "."))
			s = truncateRunes(s, n)
		}
		return FormatValue(NewStr(s), spec+"s")
	case 'r':
		s := v.Repr()
		if prec != "" {
			n, _ := strconv.Atoi(strings.TrimPrefix(prec, "."))
			s = truncateRunes(s, n)
		}
		return FormatValue(NewStr(s), spec+"s")
	case 'a':
		s := v.Repr()
		return FormatValue(NewStr(s), spec+"s")
	case 'd', 'i':
		i, ok := percentInt(v)
		if !ok {
			return "", Raisef(TypeErrorClass, "%%%c format: a real number is required, not '%s'", conv, v.TypeName())
		}
		return FormatValue(NewInt(i), spec+"d")
	case 'u':
		i, ok := percentInt(v)
		if !ok {
			return "", Raisef(TypeErrorClass, "%%u format: a real number is required, not '%s'", v.TypeName())
		}
		return FormatValue(NewInt(i), spec+"d")
	case 'x', 'X', 'o':
		i, ok := percentInt(v)
		if !ok {
			return "", Raisef(TypeErrorClass, "%%%c format: an integer is required, not '%s'", conv, v.TypeName())
		}
		return FormatValue(NewInt(i), spec+string(conv))
	case 'f', 'F', 'e', 'E', 'g', 'G':
		f, ok := percentFloat(v)
		if !ok {
			return "", Raisef(TypeErrorClass, "%%%c format: a real number is required, not '%s'", conv, v.TypeName())
		}
		if prec == "" {
			spec += ".6"
		}
		return FormatValue(NewFloat(f), spec+string(conv))
	case 'c':
		switch x := v.(type) {
		case *Str:
			if utf8.RuneCountInString(x.V) != 1 {
				return "", Raisef(TypeErrorClass, "%%c requires int or char")
			}
			return x.V, nil
		default:
			i, ok := percentInt(v)
			if !ok {
				return "", Raisef(TypeErrorClass, "%%c requires int or char")
			}
			return string(rune(i)), nil
		}
	}
	return "", Raisef(ValueErrorClass, "unsupported format character '%c' at index %d", conv, index)
}

func percentInt(v Object) (int64, bool) {
	switch x := v.(type) {
	case *Int:
		return x.V, true
	case Bool:
		return int64(boolInt(x)), true
	case *Float:
		return int64(x.V), true
	}
	return 0, false
}

func percentFloat(v Object) (float64, bool) {
	switch x := v.(type) {
	case *Int:
		return float64(x.V), true
	case Bool:
		return float64(boolInt(x)), true
	case *Float:
		return x.V, true
	}
	return 0, false
}
