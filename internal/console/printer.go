package console

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/quality-gates/mutago/v2/internal/models"

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

// TerminalReporter writes mutant results and progress for a person at a
// terminal. It implements models.RunListener and is safe for concurrent use.
type TerminalReporter struct {
	opts         *models.Options
	stdout       io.Writer
	stderr       io.Writer
	showProgress bool

	mu            sync.Mutex
	progressDrawn bool
}

// NewTerminalReporter returns a reporter that writes status lines to stdout
// and the progress line to stderr. terminal reports whether stderr is a
// terminal; the progress line is drawn only then.
func NewTerminalReporter(opts *models.Options, stdout, stderr io.Writer, terminal bool) *TerminalReporter {
	return &TerminalReporter{
		opts:   opts,
		stdout: stdout,
		stderr: stderr,
		showProgress: terminal && !opts.General.Verbose && !opts.General.Debug &&
			!opts.Config.SilentMode,
	}
}

// IsTerminal reports whether w is a character device such as a terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// OnMutantCompleted writes the mutant's status line and, where the options
// ask for it, its diff. Each mutant's output is written in one call.
func (r *TerminalReporter) OnMutantCompleted(event models.MutationEvent) {
	var buf bytes.Buffer
	r.render(&buf, event)
	if buf.Len() == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = r.stdout.Write(buf.Bytes())
}

// diffRule says when an outcome's diff is shown.
type diffRule int

const (
	diffNever     diffRule = iota
	diffWhenShown          // with the status line
	diffInDebug            // in --debug, even when the status line is hidden
)

// statusStyle is how one outcome is shown.
type statusStyle struct {
	letter byte   // --output-statuses letter
	token  string // status word to highlight
	bg     color.Attribute
	diff   diffRule
}

var (
	erroredStyle  = statusStyle{'x', UNKNOWN, color.BgMagenta, diffWhenShown}
	outcomeStyles = map[models.MutantOutcome]statusStyle{
		models.OutcomeKilled:     {'k', KILLED, color.BgGreen, diffInDebug},
		models.OutcomeEscaped:    {'e', ESCAPED, color.BgRed, diffWhenShown},
		models.OutcomeSkipped:    {'s', SKIP, color.BgYellow, diffInDebug},
		models.OutcomeNotCovered: {'n', SKIP, color.BgYellow, diffNever},
		models.OutcomeErrored:    erroredStyle,
	}
)

func (r *TerminalReporter) render(w io.Writer, event models.MutationEvent) {
	style, ok := outcomeStyles[event.Outcome]
	if !ok {
		style = erroredStyle
	}
	shown := r.statusVisible(style.letter)
	if shown {
		printStatus(w, event.Mutant.ProcessOutput, style.token, style.bg)
	}
	if event.Outcome == models.OutcomeSkipped && r.opts.General.Verbose {
		fmt.Fprintln(w, "Mutation did not compile")
	}
	if r.diffVisible(style.diff, shown) && !r.opts.General.NoDiffs && event.Mutant.Diff != "" {
		printDiff(w, event.Mutant.Diff)
	}
}

func (r *TerminalReporter) diffVisible(rule diffRule, shown bool) bool {
	switch rule {
	case diffWhenShown:
		return shown
	case diffInDebug:
		return r.opts.General.Debug
	default:
		return false
	}
}

// statusVisible reports whether the options show mutants whose status has
// the --output-statuses letter.
func (r *TerminalReporter) statusVisible(letter byte) bool {
	opts := r.opts
	if opts.Config.SilentMode {
		return false
	}
	if opts.General.OutputStatuses != "" {
		return strings.IndexByte(opts.General.OutputStatuses, letter) >= 0
	}
	if opts.General.Quiet {
		return letter == 'e'
	}
	return true
}

// OnProgress redraws the progress line when stderr is a terminal.
func (r *TerminalReporter) OnProgress(stats models.Stats) {
	if !r.showProgress {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	total := stats.KilledCount + stats.EscapedCount + stats.ErrorCount + stats.SkippedCount + stats.NotCoveredCount
	fmt.Fprintf(r.stderr, "\rProcessed %d mutants (%d killed, %d escaped, %d not covered, %d errored)",
		total, stats.KilledCount, stats.EscapedCount, stats.NotCoveredCount, stats.ErrorCount)
	r.progressDrawn = true
}

// OnSummary clears the progress line so the results that follow start on a
// clean line.
func (r *TerminalReporter) OnSummary(*models.Report) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.progressDrawn {
		fmt.Fprint(r.stderr, "\r\x1b[K")
		r.progressDrawn = false
	}
}

// printStatus writes out with its first status token highlighted on bg,
// followed by a frame line.
func printStatus(w io.Writer, out, status string, bg color.Attribute) {
	highlight := color.New(color.FgHiWhite, bg).SprintfFunc()
	fmt.Fprint(w, strings.Replace(out, status, highlight(status), 1))
	_, _ = color.New(color.FgBlue).Fprintln(w, frameLine)
}

// printDiff writes diff with added lines in green and removed lines in red.
func printDiff(w io.Writer, diff string) {
	green := color.New(color.FgHiWhite).Add(color.BgGreen)
	red := color.New(color.FgHiWhite).Add(color.BgRed)

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			_, _ = green.Fprintln(w, line)
		case strings.HasPrefix(line, "-"):
			_, _ = red.Fprintln(w, line)
		default:
			fmt.Fprintln(w, line)
		}
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
