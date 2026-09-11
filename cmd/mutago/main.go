package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/internal/engine"
	"github.com/quality-gates/mutago/v2/internal/importing"
	"github.com/quality-gates/mutago/v2/internal/models"
	"github.com/quality-gates/mutago/v2/internal/parser"
	"github.com/quality-gates/mutago/v2/internal/version"
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

	if len(opts.Remaining.Targets) == 0 {
		if flagName, val, ok := findSwallowedTargetFlag(p, args); ok {
			return true, exitError("flag %q consumed %q as its argument, leaving no targets. Use \"%s=<value>\" or pass targets after the flag value.", flagName, val, flagName)
		}
	}

	if opts.General.Debug {
		opts.General.Verbose = true
	}

	opts.ApplyConfigDefaults()

	if handled, code := loadConfigFile(opts); handled {
		return true, code
	}

	return false, 0
}

// findSwallowedTargetFlag checks if any value-taking flag in args consumed a value
// that looks like a target package, directory, or source file.
func findSwallowedTargetFlag(p *flags.Parser, args []string) (string, string, bool) {
	optsByLong, optsByShort := indexOptions(p)

	for i, arg := range args {
		nextArg := ""
		hasNext := i+1 < len(args)
		if hasNext {
			nextArg = args[i+1]
		}
		opt, val := parseOptionArg(arg, nextArg, hasNext, optsByLong, optsByShort)
		if opt != nil && isTargetLike(val) {
			flagName := "--" + opt.LongName
			if opt.LongName == "" && opt.ShortName != 0 {
				flagName = "-" + string(opt.ShortName)
			}
			return flagName, val, true
		}
	}

	return "", "", false
}

func indexOptions(p *flags.Parser) (map[string]*flags.Option, map[rune]*flags.Option) {
	optsByLong := make(map[string]*flags.Option)
	optsByShort := make(map[rune]*flags.Option)

	var visit func(g *flags.Group)
	visit = func(g *flags.Group) {
		for _, o := range g.Options() {
			if o.LongName != "" {
				optsByLong[o.LongName] = o
			}
			if o.ShortName != 0 {
				optsByShort[o.ShortName] = o
			}
		}
		for _, sub := range g.Groups() {
			visit(sub)
		}
	}
	visit(p.Group)
	return optsByLong, optsByShort
}

func parseOptionArg(arg, nextArg string, hasNext bool, optsByLong map[string]*flags.Option, optsByShort map[rune]*flags.Option) (*flags.Option, string) {
	if strings.HasPrefix(arg, "--") {
		return parseLongOption(strings.TrimPrefix(arg, "--"), nextArg, hasNext, optsByLong)
	}
	if strings.HasPrefix(arg, "-") && len(arg) == 2 {
		return parseShortOption(rune(arg[1]), nextArg, hasNext, optsByShort)
	}
	return nil, ""
}

func parseLongOption(name, nextArg string, hasNext bool, optsByLong map[string]*flags.Option) (*flags.Option, string) {
	if eqIdx := strings.Index(name, "="); eqIdx != -1 {
		return getActiveOption(optsByLong[name[:eqIdx]], name[eqIdx+1:])
	}
	if hasNext {
		return getActiveOption(optsByLong[name], nextArg)
	}
	return nil, ""
}

func parseShortOption(r rune, nextArg string, hasNext bool, optsByShort map[rune]*flags.Option) (*flags.Option, string) {
	if hasNext {
		return getActiveOption(optsByShort[r], nextArg)
	}
	return nil, ""
}

func getActiveOption(o *flags.Option, val string) (*flags.Option, string) {
	if o != nil && o.IsSet() && !o.IsSetDefault() {
		return o, val
	}
	return nil, ""
}

// isTargetLike reports whether val appears to be a target package pattern, directory, or source file.
func isTargetLike(val string) bool {
	if val == "" || strings.HasPrefix(val, "-") {
		return false
	}
	if isWildcardPattern(val) || isExistingTarget(val) {
		return true
	}
	if strings.HasSuffix(val, ".go") {
		return true
	}
	return isPotentialPathTarget(val)
}

func isWildcardPattern(val string) bool {
	return val == "..." || strings.HasSuffix(val, "/...")
}

func isExistingTarget(val string) bool {
	info, err := os.Stat(val)
	return err == nil && (info.IsDir() || strings.HasSuffix(val, ".go"))
}

func isPotentialPathTarget(val string) bool {
	ext := strings.ToLower(filepath.Ext(val))
	if ext != "" && ext != ".go" {
		return false
	}
	return strings.HasPrefix(val, "./") || strings.HasPrefix(val, "../")
}

// isCompletion reports whether mutago was invoked for shell completion.
func isCompletion() bool {
	return len(os.Getenv("GO_FLAGS_COMPLETION")) > 0
}

// handleEarlyExitFlags prints metadata and exits before mutation work.
// Returns (true, exitCode) when an early-exit flag was handled.
func handleEarlyExitFlags(opts *models.Options, p *flags.Parser, args []string) (bool, int) {
	if opts.General.Version {
		fmt.Printf("mutago %s\n", version.Version)

		return true, returnOk
	}
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
	if err := dec.Decode(&opts.Config); err != nil && !errors.Is(err, io.EOF) {
		return true, exitError("Could not parse config file %q: %v", opts.General.Config, err)
	}
	return false, 0
}

func exitError(format string, args ...interface{}) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, format)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	}

	return returnError
}

func mainCmd(args []string) int {
	opts := models.NewOptions()

	if exit, exitCode := checkArguments(args, opts); exit {
		return exitCode
	}

	targets := importing.ResolveTargets(opts.Remaining.Targets, opts)
	if len(targets.Files) == 0 {
		return exitError("Could not find any suitable Go source files")
	}

	bl, err := baseline.Load(opts.Baseline.File)
	if err != nil {
		return exitError("Cannot load baseline: %v", err)
	}

	if handled, code := handleInfoFlags(opts, targets.Files); handled {
		return code
	}

	return runMutationTesting(opts, bl, targets)
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

func runMutationTesting(opts *models.Options, bl *baseline.File, targets importing.ResolvedTargets) int {
	e := &engine.Engine{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()

	res, err := e.RunResolved(ctx, opts, bl, targets)
	if err != nil {
		return exitError(err.Error())
	}
	return res.ExitCode
}

func main() {
	os.Exit(mainCmd(os.Args[1:]))
}
