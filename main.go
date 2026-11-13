package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	switch arg1 := sliceAt(os.Args, 1); arg1 {

	case "-h":
		fmt.Println("usage: gpython [-h | -c cmd | -m mod | file | -] [arg]")

	case "-c":
		arg2 := sliceAt(os.Args, 2)
		if arg2 == "" {
			return
		}

		runCmd(arg2, sliceFrom(os.Args, 3))

	case "-m":
		arg2 := sliceAt(os.Args, 2)
		if arg2 == "" {
			return
		}

		runMod(arg2, sliceFrom(os.Args, 3))

	default:
		runFile(arg1, sliceFrom(os.Args, 2))

	case "":
		runREPL()

	}
}

func sliceAt[T any](s []T, i int) T {
	var v T

	if i >= 0 && i < len(s) {
		v = s[i]
	}

	return v
}

func sliceFrom[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return nil
	}

	return s[i:]
}

func runCmd(cmd string, args []string) {
	fmt.Println("python -c", cmd, strings.Join(args, " ")) //placeholder
}

func runMod(mod string, args []string) {
	fmt.Println("python -m", mod, strings.Join(args, " ")) //placeholder
}

func runFile(file string, args []string) {
	fmt.Println("python", file, strings.Join(args, " ")) //placeholder
}

func runREPL() {
	fmt.Println("python") //placeholder
}
