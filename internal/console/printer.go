package console

import (
	"fmt"
	"github.com/quality-gates/mutago/v2/internal/models"
	"log"
	"strings"

	"github.com/fatih/color"
)

// for colouring
const (
	KILLED  = "KILLED"
	ESCAPED = "ESCAPED"
	SKIP    = "SKIP"
	UNKNOWN = "UNKNOWN"
)

var (
	length    = 150
	frameLine = strings.Repeat("-", length)
)

// PrintKilled prints killed mutants in green (the test suite caught the mutation).
func PrintKilled(out string) {
	killed := color.New(color.FgHiWhite, color.BgGreen).SprintfFunc()
	out = strings.Replace(out, KILLED, killed(KILLED), 1)
	fmt.Print(out)
	color.Blue(frameLine)
}

// PrintEscaped prints escaped mutants in red (the test suite missed the mutation).
func PrintEscaped(out string) {
	escaped := color.New(color.FgHiWhite, color.BgRed).SprintfFunc()
	out = strings.Replace(out, ESCAPED, escaped(ESCAPED), 1)
	fmt.Print(out)
	color.Blue(frameLine)
}

// PrintSkip prints in yellow
func PrintSkip(out string) {
	skip := color.New(color.FgHiWhite, color.BgYellow).SprintfFunc()
	out = strings.Replace(out, SKIP, skip(SKIP), 1)
	fmt.Print(out)
	color.Blue(frameLine)
}

// PrintUnknown prints in magenta
func PrintUnknown(out string) {
	unknown := color.New(color.FgHiWhite, color.BgMagenta).SprintfFunc()
	out = strings.Replace(out, UNKNOWN, unknown(UNKNOWN), 1)
	fmt.Print(out)
	color.Blue(frameLine)
}

// PrintDiff prints colorful diff
func PrintDiff(diff []byte) {
	green := color.New(color.FgHiWhite).Add(color.BgGreen)
	red := color.New(color.FgHiWhite).Add(color.BgRed)

	for _, line := range strings.Split(string(diff), "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			printColoredLine(green, line)
		case strings.HasPrefix(line, "-"):
			printColoredLine(red, line)
		default:
			fmt.Println(line)
		}
	}
}

// printColoredLine prints line using c, logging any write error.
func printColoredLine(c *color.Color, line string) {
	if _, err := c.Println(line); err != nil {
		log.Printf("Error printing output: %s", err)
	}
}

// Debug prints formatted debug messages when debug mode is enabled in options.
func Debug(opts *models.Options, format string, args ...interface{}) {
	if opts.General.Debug {
		fmt.Printf(format+"\n", args...)
	}
}

// Verbose prints formatted messages when either verbose or debug mode is enabled.
func Verbose(opts *models.Options, format string, args ...interface{}) {
	if opts.General.Verbose || opts.General.Debug {
		fmt.Printf(format+"\n", args...)
	}
}
