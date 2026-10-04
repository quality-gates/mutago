package engine

import (
	"fmt"
	"strings"
)

// testFlag is one --test-flags token written as -name, --name or -name=value.
type testFlag struct {
	name     string
	value    string
	hasValue bool
}

// splitTestFlag parses arg. ok is false for tokens that are not flags, such
// as the value that follows a flag written as "-name value".
func splitTestFlag(arg string) (flag testFlag, ok bool) {
	if !strings.HasPrefix(arg, "-") {
		return testFlag{}, false
	}
	raw := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
	flag.name, flag.value, flag.hasValue = strings.Cut(raw, "=")
	return flag, true
}

// hasTestFlag reports whether the user's test flags set name in any form.
func hasTestFlag(testFlags []string, name string) bool {
	for _, flag := range testFlags {
		if got, ok := splitTestFlag(flag); ok && got.name == name {
			return true
		}
	}
	return false
}

// testCountValue returns the value of a -count flag at testFlags[index].
func testCountValue(testFlags []string, index int) (string, bool) {
	flag, ok := splitTestFlag(testFlags[index])
	if !ok || flag.name != "count" {
		return "", false
	}
	if flag.hasValue {
		return flag.value, true
	}
	if index+1 >= len(testFlags) {
		return "", false
	}
	return testFlags[index+1], true
}

func uncachedTestFlags(testFlags []string) []string {
	if hasTestFlag(testFlags, "count") {
		return testFlags
	}
	return append(append([]string{}, testFlags...), "-count=1")
}

// isBuildFlag reports whether name is a build flag: one that changes which
// files are compiled or how, so every command that lists, compiles or runs
// tests must see it.
func isBuildFlag(name string) bool {
	switch name {
	case "race", "msan", "asan", "trimpath", "tags", "gcflags", "asmflags", "ldflags", "mod", "modfile":
		return true
	default:
		return false
	}
}

// takesValue reports whether a go-command-only flag (a build flag or -vet)
// may take its value as the next token.
func takesValue(name string) bool {
	switch name {
	case "tags", "vet", "gcflags", "asmflags", "ldflags", "mod", "modfile":
		return true
	default:
		return false
	}
}

func isRunnerBool(name string) bool {
	switch name {
	case "v", "verbose", "short", "failfast":
		return true
	default:
		return false
	}
}

func isRunnerValue(name string) bool {
	switch name {
	case "count", "parallel", "shuffle", "cpu", "timeout", "run", "bench", "skip":
		return true
	default:
		return false
	}
}

func hasNextValue(args []string, i int) bool {
	if i+1 >= len(args) {
		return false
	}
	if strings.HasPrefix(args[i+1], "-") {
		return false
	}
	return true
}

// valueTokens is how many tokens after args[i] belong to its go-command flag.
// As in the go command, the next token is the value even when it starts with
// a dash, as in "-ldflags -s".
func valueTokens(args []string, i int, flag testFlag) int {
	if flag.hasValue || !takesValue(flag.name) || i+1 >= len(args) {
		return 0
	}
	return 1
}

// buildTestFlags returns the build flags in testFlags, with their values.
// Commands that only list packages or tests take these and nothing else.
func buildTestFlags(testFlags []string) []string {
	var flags []string
	for i := 0; i < len(testFlags); i++ {
		flag, ok := splitTestFlag(testFlags[i])
		if !ok || !isBuildFlag(flag.name) {
			continue
		}
		advance := valueTokens(testFlags, i, flag)
		flags = append(flags, testFlags[i:i+advance+1]...)
		i += advance
	}
	return flags
}

func formatRunnerBool(flag testFlag) string {
	name := flag.name
	if name == "verbose" {
		name = "v"
	}
	if flag.hasValue {
		return fmt.Sprintf("-test.%s=%s", name, flag.value)
	}
	return fmt.Sprintf("-test.%s=true", name)
}

func formatRunnerValue(args []string, i int, flag testFlag) (int, string, bool) {
	if flag.hasValue {
		return 0, fmt.Sprintf("-test.%s=%s", flag.name, flag.value), true
	}
	if hasNextValue(args, i) {
		return 1, fmt.Sprintf("-test.%s=%s", flag.name, args[i+1]), true
	}
	return 0, fmt.Sprintf("-test.%s", flag.name), true
}

// translateFlag maps args[i] to the form a compiled test binary accepts. It
// returns how many extra tokens it consumed, the translated flag, and whether
// to keep it. Build flags and -vet were applied at compile time and are
// dropped.
func translateFlag(args []string, i int) (int, string, bool) {
	flag, ok := splitTestFlag(args[i])
	if !ok {
		return 0, args[i], true
	}
	if isBuildFlag(flag.name) || flag.name == "vet" {
		return valueTokens(args, i, flag), "", false
	}
	if isRunnerBool(flag.name) {
		return 0, formatRunnerBool(flag), true
	}
	if isRunnerValue(flag.name) {
		return formatRunnerValue(args, i, flag)
	}
	return 0, args[i], true
}

// testBinaryFlags translates the user's test flags for a compiled test
// binary, which accepts only -test.* runner flags.
func testBinaryFlags(testFlags []string) []string {
	flags := make([]string, 0, len(testFlags))
	for i := 0; i < len(testFlags); i++ {
		advance, flag, keep := translateFlag(testFlags, i)
		i += advance
		if keep {
			flags = append(flags, flag)
		}
	}
	return flags
}
