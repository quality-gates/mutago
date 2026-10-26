package console

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/quality-gates/mutago/v2/internal/models"
)

const testDiff = "--- Original\n+++ New\n-a + b\n+a - b"

func init() {
	color.NoColor = true
}

func event(outcome models.MutantOutcome, out string) models.MutationEvent {
	return models.MutationEvent{
		Mutant:  models.Mutant{ProcessOutput: out, Diff: testDiff},
		Outcome: outcome,
	}
}

func render(t *testing.T, opts *models.Options, ev models.MutationEvent) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	NewTerminalReporter(opts, &stdout, &stderr, false).OnMutantCompleted(ev)
	if stderr.Len() != 0 {
		t.Fatalf("status output went to stderr: %q", stderr.String())
	}
	return stdout.String()
}

func TestTerminalReporterWritesStatusLineAndFrame(t *testing.T) {
	cases := map[models.MutantOutcome]string{
		models.OutcomeKilled:     "KILLED f.go:1 (m)\n",
		models.OutcomeEscaped:    "ESCAPED f.go:1 (m)\n",
		models.OutcomeSkipped:    "SKIP f.go:1 (m)\n",
		models.OutcomeErrored:    "UNKNOWN exit code for f.go:1 (m)\n",
		models.OutcomeNotCovered: "NOT COVERED f.go:1 (m)\n",
	}
	for outcome, line := range cases {
		opts := &models.Options{}
		opts.General.NoDiffs = true
		got := render(t, opts, event(outcome, line))
		if want := line + frameLine + "\n"; got != want {
			t.Errorf("outcome %v: got %q, want %q", outcome, got, want)
		}
	}
}

func TestTerminalReporterHighlightsStatusToken(t *testing.T) {
	color.NoColor = false
	defer func() { color.NoColor = true }()
	opts := &models.Options{}
	opts.General.NoDiffs = true
	got := render(t, opts, event(models.OutcomeEscaped, "ESCAPED f.go:1 (m)\n"))
	want := color.New(color.FgHiWhite, color.BgRed).Sprint(ESCAPED)
	if !strings.HasPrefix(got, want+" f.go:1 (m)\n") {
		t.Fatalf("got %q, want the status highlighted as %q", got, want)
	}
}

func TestTerminalReporterStatusVisibility(t *testing.T) {
	silent := &models.Options{}
	silent.Config.SilentMode = true
	quiet := &models.Options{}
	quiet.General.Quiet = true
	statuses := &models.Options{}
	statuses.General.OutputStatuses = "kn"

	cases := []struct {
		name    string
		opts    *models.Options
		outcome models.MutantOutcome
		shown   bool
	}{
		{"silent hides escaped", silent, models.OutcomeEscaped, false},
		{"quiet shows escaped", quiet, models.OutcomeEscaped, true},
		{"quiet hides killed", quiet, models.OutcomeKilled, false},
		{"statuses show killed", statuses, models.OutcomeKilled, true},
		{"statuses show not covered", statuses, models.OutcomeNotCovered, true},
		{"statuses hide escaped", statuses, models.OutcomeEscaped, false},
		{"statuses hide skipped", statuses, models.OutcomeSkipped, false},
		{"statuses hide errored", statuses, models.OutcomeErrored, false},
	}
	for _, c := range cases {
		c.opts.General.NoDiffs = true
		got := render(t, c.opts, event(c.outcome, "STATUS f.go:1\n"))
		if shown := got != ""; shown != c.shown {
			t.Errorf("%s: output %q", c.name, got)
		}
	}
}

func TestTerminalReporterDiffs(t *testing.T) {
	normal := &models.Options{}
	debug := &models.Options{}
	debug.General.Debug = true
	noDiffs := &models.Options{}
	noDiffs.General.NoDiffs = true
	debugNoDiffs := &models.Options{}
	debugNoDiffs.General.Debug = true
	debugNoDiffs.General.NoDiffs = true
	quietDebug := &models.Options{}
	quietDebug.General.Debug = true
	quietDebug.General.Quiet = true

	cases := []struct {
		name    string
		opts    *models.Options
		outcome models.MutantOutcome
		diff    bool
	}{
		{"escaped shows diff", normal, models.OutcomeEscaped, true},
		{"no-diffs hides escaped diff", noDiffs, models.OutcomeEscaped, false},
		{"errored shows diff", normal, models.OutcomeErrored, true},
		{"no-diffs hides errored diff", noDiffs, models.OutcomeErrored, false},
		{"killed hides diff", normal, models.OutcomeKilled, false},
		{"debug shows killed diff", debug, models.OutcomeKilled, true},
		{"debug no-diffs hides killed diff", debugNoDiffs, models.OutcomeKilled, false},
		{"debug shows hidden killed diff", quietDebug, models.OutcomeKilled, true},
		{"skipped hides diff", normal, models.OutcomeSkipped, false},
		{"debug shows skipped diff", debug, models.OutcomeSkipped, true},
		{"debug no-diffs hides skipped diff", debugNoDiffs, models.OutcomeSkipped, false},
		{"not covered hides diff", debug, models.OutcomeNotCovered, false},
	}
	for _, c := range cases {
		got := render(t, c.opts, event(c.outcome, "STATUS f.go:1\n"))
		if hasDiff := strings.Contains(got, "+a - b\n"); hasDiff != c.diff {
			t.Errorf("%s: output %q", c.name, got)
		}
	}
}

func TestTerminalReporterDiffLines(t *testing.T) {
	got := render(t, &models.Options{}, event(models.OutcomeEscaped, "ESCAPED f.go:1\n"))
	want := "ESCAPED f.go:1\n" + frameLine + "\n" + testDiff + "\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTerminalReporterNoDiffWithoutDiffText(t *testing.T) {
	ev := event(models.OutcomeEscaped, "ESCAPED f.go:1\n")
	ev.Mutant.Diff = ""
	got := render(t, &models.Options{}, ev)
	if want := "ESCAPED f.go:1\n" + frameLine + "\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTerminalReporterVerboseSkipNote(t *testing.T) {
	verbose := &models.Options{}
	verbose.General.Verbose = true
	verbose.General.OutputStatuses = "e"
	got := render(t, verbose, event(models.OutcomeSkipped, "SKIP f.go:1\n"))
	if got != "Mutation did not compile\n" {
		t.Fatalf("got %q", got)
	}
	quiet := &models.Options{}
	quiet.General.OutputStatuses = "e"
	if got := render(t, quiet, event(models.OutcomeSkipped, "SKIP f.go:1\n")); got != "" {
		t.Fatalf("non-verbose skip note: %q", got)
	}
}

func TestTerminalReporterProgress(t *testing.T) {
	stats := models.Stats{KilledCount: 3, EscapedCount: 2, NotCoveredCount: 1, ErrorCount: 1, SkippedCount: 1}
	const line = "\rProcessed 8 mutants (3 killed, 2 escaped, 1 not covered, 1 errored)"

	var stdout, stderr bytes.Buffer
	r := NewTerminalReporter(&models.Options{}, &stdout, &stderr, true)
	r.OnProgress(stats)
	r.OnSummary(&models.Report{})
	r.OnSummary(&models.Report{})
	if got, want := stderr.String(), line+"\r\x1b[K"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("progress went to stdout: %q", stdout.String())
	}
}

func TestTerminalReporterHidesProgress(t *testing.T) {
	verbose := &models.Options{}
	verbose.General.Verbose = true
	debug := &models.Options{}
	debug.General.Debug = true
	silent := &models.Options{}
	silent.Config.SilentMode = true
	cases := []struct {
		name     string
		opts     *models.Options
		terminal bool
	}{
		{"not a terminal", &models.Options{}, false},
		{"verbose", verbose, true},
		{"debug", debug, true},
		{"silent", silent, true},
	}
	for _, c := range cases {
		var stderr bytes.Buffer
		r := NewTerminalReporter(c.opts, &bytes.Buffer{}, &stderr, c.terminal)
		r.OnProgress(models.Stats{KilledCount: 1})
		r.OnSummary(&models.Report{})
		if stderr.Len() != 0 {
			t.Errorf("%s: stderr = %q", c.name, stderr.String())
		}
	}
}

func TestIsTerminal(t *testing.T) {
	if IsTerminal(&bytes.Buffer{}) {
		t.Fatal("a buffer is not a terminal")
	}
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Fatal("a regular file is not a terminal")
	}
}
