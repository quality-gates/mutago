package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/internal/engine"
	"github.com/quality-gates/mutago/v2/internal/importing"
	"github.com/quality-gates/mutago/v2/internal/models"
	"github.com/quality-gates/mutago/v2/internal/parser"
	"github.com/quality-gates/mutago/v2/mutator"

	_ "github.com/quality-gates/mutago/v2/mutator/arithmetic"
	_ "github.com/quality-gates/mutago/v2/mutator/branch"
	_ "github.com/quality-gates/mutago/v2/mutator/composite"
	_ "github.com/quality-gates/mutago/v2/mutator/concurrency"
	_ "github.com/quality-gates/mutago/v2/mutator/conditional"
	_ "github.com/quality-gates/mutago/v2/mutator/expression"
	_ "github.com/quality-gates/mutago/v2/mutator/loop"
	_ "github.com/quality-gates/mutago/v2/mutator/numbers"
	_ "github.com/quality-gates/mutago/v2/mutator/select"
	_ "github.com/quality-gates/mutago/v2/mutator/statement"
)

const (
	returnOk                 = 0
	returnHelp               = 1
	returnBashCompletion     = 2
	returnError              = 3
	returnMsiThresholdNotMet = 4
)

func checkArguments(args []string, opts *models.Options) (bool, int) {
	p := flags.NewNamedParser("mutago", flags.None)

	p.ShortDescription = "Mutation testing for Go source code"

	if _, err := p.AddGroup("mutago", "mutago arguments", opts); err != nil {
		return true, exitError(err.Error())
	}

	_, err := p.ParseArgs(args)

	// --help and --list-mutators print and exit before any parse error is reported.
	if handled, code := handleEarlyExitFlags(opts, p, args); handled {
		return true, code
	}

	if err != nil {
		return true, exitError(err.Error())
	}

	if isCompletion() {
		return true, returnBashCompletion
	}

	if opts.General.Debug {
		opts.General.Verbose = true
	}

	if handled, code := loadConfigFile(opts); handled {
		return true, code
	}

	return false, 0
}

// isCompletion reports whether mutago was invoked for shell completion.
func isCompletion() bool {
	return len(os.Getenv("GO_FLAGS_COMPLETION")) > 0
}

// handleEarlyExitFlags handles --help and --list-mutators, which print and exit
// before any mutation work. Returns (true, exitCode) when one applied.
func handleEarlyExitFlags(opts *models.Options, p *flags.Parser, args []string) (bool, int) {
	if (opts.General.Help || len(args) == 0) && !isCompletion() {
		p.WriteHelp(os.Stdout)

		return true, returnOk // exit 0 is conventional for --help
	}
	if opts.Mutator.ListMutators {
		for _, name := range mutator.List() {
			fmt.Println(name)
		}

		return true, returnOk
	}
	return false, 0
}

// loadConfigFile merges a YAML config file into opts when --config is set.
// Returns (true, exitCode) on failure, (false, 0) otherwise.
func loadConfigFile(opts *models.Options) (bool, int) {
	if opts.General.Config == "" {
		return false, 0
	}
	yamlFile, err := os.ReadFile(opts.General.Config)
	if err != nil {
		return true, exitError("Could not read config file: %q", opts.General.Config)
	}
	dec := yaml.NewDecoder(bytes.NewReader(yamlFile))
	dec.KnownFields(true)
	if err := dec.Decode(&opts.Config); err != nil {
		return true, exitError("Could not parse config file %q: %v", opts.General.Config, err)
	}
	return false, 0
}

func exitError(format string, args ...interface{}) int {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)

	return returnError
}

func mainCmd(args []string) int {
	opts := &models.Options{}

	if exit, exitCode := checkArguments(args, opts); exit {
		return exitCode
	}

	files := importing.FilesOfArgs(opts.Remaining.Targets, opts)
	if len(files) == 0 {
		return exitError("Could not find any suitable Go source files")
	}

	bl, err := baseline.Load(opts.Baseline.File)
	if err != nil {
		return exitError("Cannot load baseline: %v", err)
	}

	if handled, code := handleInfoFlags(opts, files); handled {
		return code
	}

	return runMutationTesting(opts, bl)
}

func handleInfoFlags(opts *models.Options, files []string) (bool, int) {
	if opts.Files.ListFiles {
		for _, file := range files {
			fmt.Println(file)
		}
		return true, returnOk
	}
	if opts.Files.PrintAST {
		for _, file := range files {
			fmt.Println(file)
			src, _, err := parser.ParseFile(file)
			if err != nil {
				return true, exitError("Could not open file %q: %v", file, err)
			}
			mutago.PrintWalk(src)
			fmt.Println()
		}
		return true, returnOk
	}
	return false, 0
}

func runMutationTesting(opts *models.Options, bl *baseline.File) int {
	e := &engine.Engine{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	res, err := e.Run(context.Background(), opts, bl)
	if err != nil {
		return exitError(err.Error())
	}
	return res.ExitCode
}

func main() {
	os.Exit(mainCmd(os.Args[1:]))
}
