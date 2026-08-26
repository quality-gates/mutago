package reportmaker

import (
	_ "embed" // for embedding report template
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/internal/models"
)

// mutatorDescriptions maps mutator names to plain-English explanations.
var mutatorDescriptions = map[string]string{
	"arithmetic/assign_invert":     "Inverts a compound assignment operator (e.g. += becomes -=)",
	"arithmetic/assignment":        "Swaps an assignment operator for a different one (e.g. = becomes +=)",
	"arithmetic/base":              "Swaps an arithmetic operator (+, -, *, /) for a different one",
	"arithmetic/bitwise":           "Swaps a bitwise operator (&, |, ^, <<, >>) for a different one",
	"arithmetic/negate":            "Negates a numeric value by prepending a unary minus",
	"branch/case":                  "Replaces a switch case body with a noop, making that branch a no-op",
	"branch/if":                    "Removes an if-block body so the condition becomes a no-op",
	"branch/else":                  "Removes an else-block body",
	"composite/field-clear":        "Drops a keyed field from a composite literal, leaving it at its zero value",
	"concurrency/goroutine-remove": "Converts a goroutine launch to a regular blocking call, removing concurrency",
	"conditional/bool-literal":     "Swaps a hardcoded boolean literal (true↔false) in an assignment or function argument",
	"conditional/negated":          "Negates a boolean or comparison condition (e.g. == becomes !=, < becomes >=)",
	"conditional/not":              "Removes the logical-NOT operator from a negated condition (!x becomes x)",
	"expression/comparison":        "Replaces a comparison operator with a boundary variant (e.g. < becomes <=)",
	"expression/context-nil":       "Replaces a context argument with nil, bypassing deadline and cancellation propagation",
	"expression/error-guard":       "Removes an error-guard block so the function continues even on error",
	"expression/errorf-wrap":       "Downgrades an error-wrapping %w verb to %v, so the returned error no longer wraps its cause",
	"expression/recover-clear":     "Neutralises a recover() call (recover() becomes any(nil)) so a panic propagates instead of being recovered",
	"expression/string-literal":    "Replaces a non-empty string literal in an == or != comparison with an empty string",
	"expression/logical":           "Swaps a logical operator (&& becomes ||, or vice versa)",
	"expression/remove":            "Removes an expression statement entirely, dropping its side effect",
	"loop/break":                   "Removes a break statement, potentially causing an infinite loop",
	"loop/condition":               "Changes the loop's termination condition",
	"loop/range_break":             "Removes a break statement inside a range loop",
	"numbers/decrementer":          "Decrements a numeric literal by 1",
	"numbers/float-negate":         "Negates a floating-point literal (e.g. 1.5 becomes -1.5)",
	"numbers/incrementer":          "Increments a numeric literal by 1",
	"select/case-remove":           "Removes a case from a select statement, reducing channel handling paths",
	"select/default-remove":        "Removes the default case from a select statement",
	"statement/defer-remove":       "Turns a deferred call into an immediate call, removing the guaranteed-at-exit timing",
	"statement/remove":             "Removes a statement entirely, dropping its side effect or return value",
	"statement/remove-self-assign": "Removes a self-assignment statement (e.g. x = x)",
	"statement/return":             "Replaces a return value with its zero value",
}

// killHints maps mutator names to heuristic advice for writing a killing test.
var killHints = map[string]string{
	"arithmetic/assign_invert":     "Write a test that asserts the accumulated result after the operation — inverting += to -= produces a different total",
	"arithmetic/assignment":        "Write a test that asserts the exact value after the assignment — different operators produce different results",
	"arithmetic/base":              "Write a test with specific numeric inputs and assert the exact output — boundary values expose operator swaps best",
	"arithmetic/bitwise":           "Write a test with inputs where different bitwise operators produce distinct results and assert the exact output",
	"arithmetic/negate":            "Write a test that asserts the sign or magnitude of the result — negation flips positive to negative",
	"branch/case":                  "Write a test that enters this switch case and asserts the output or side effect it produces",
	"branch/if":                    "Write a test that enters this branch and asserts the output or side effect it produces",
	"branch/else":                  "Write a test where the else path is taken and assert its expected result",
	"composite/field-clear":        "Think about what a caller observes if this field were left unset. Write a test that drives the code via its public API and asserts the behaviour that depends on this field's value",
	"concurrency/goroutine-remove": "Write a test that asserts concurrent behaviour — e.g. a channel receive, a timing constraint, or a race-detector hit",
	"conditional/bool-literal":     "Think about what a caller would observe if this flag were wrong. Write a test that drives the code via its public API with both values and asserts the different outcomes that a correct caller should see",
	"conditional/negated":          "Write tests that exercise both the true and false paths of this condition and assert different outcomes for each",
	"conditional/not":              "Think about what changes when the condition is satisfied vs not. Write tests that drive the code through both paths via its public API and assert the distinct outcomes a caller would see",
	"expression/comparison":        "Write tests at the exact boundary value — one that satisfies the condition and one that doesn't — and assert different outcomes",
	"expression/context-nil":       "Write a test that passes a context with a deadline or cancel and asserts the function respects it",
	"expression/error-guard":       "Write a test that triggers the error condition and asserts the function returns the error rather than continuing",
	"expression/errorf-wrap":       "Write a test that asserts errors.Is or errors.As against the wrapped cause — downgrading %w to %v breaks unwrapping while leaving the message identical",
	"expression/recover-clear":     "Write a test that triggers the panic and asserts the recovery behaviour a caller would observe — e.g. the function returns an error instead of crashing the process",
	"expression/string-literal":    "Think about what a caller expects when this string matches vs doesn't. Write a test that supplies a value that should match, and one that should not, and asserts the different outcomes a caller would see through the public API",
	"expression/logical":           "Write tests where only one operand is true/false so && and || produce different outcomes",
	"expression/remove":            "Write a test that asserts the side effect or state change this expression produces",
	"loop/break":                   "Write a test that asserts the loop terminates at the right iteration",
	"loop/condition":               "Write a test with a known input and assert the exact number of loop iterations or the final state",
	"loop/range_break":             "Write a test that asserts the loop stops at the correct element",
	"numbers/decrementer":          "Write a test that asserts the exact numeric value",
	"numbers/float-negate":         "Write a test that asserts the sign or exact value of the float result",
	"numbers/incrementer":          "Write a test that asserts the exact numeric value — off-by-one mutations are killed by precise equality assertions",
	"select/case-remove":           "Write a test that sends on the removed channel case and asserts the expected receive or resulting action",
	"select/default-remove":        "Write a test where no channel is ready (the default path) and assert its behaviour",
	"statement/defer-remove":       "Think about what a caller would observe if cleanup happened too early. Write a test that checks the state visible to a caller after the function returns — e.g. a file is closed, a lock is released, a span is finished — and ensure it only happens at the right time",
	"statement/remove":             "Write a test that asserts the side effect or state change this statement produces",
	"statement/remove-self-assign": "Write a test that asserts the value is unchanged after a self-assignment — any mutation would alter it",
	"statement/return":             "Write a test that asserts the exact return value — zero-value substitutions are caught by equality assertions",
}

//go:embed templates/report.html.gotpl
var reportTmpl string

var funcMap = template.FuncMap{
	"splitDiff": func(diff string) []string {
		return strings.Split(diff, "\n")
	},
	"hasPrefix": strings.HasPrefix,
}

// MakeHTMLReport is a function for creating an HTML report based on a stripped-down version of the models.Report model (not all fields are used)
func MakeHTMLReport(report models.Report) error {
	// Convert 0–1 ratio to percentage for the HTML template.
	report.Stats.Msi = math.Round(report.Stats.Msi*10_000) / 100
	groupedMutants := groupEscapedMutants(report.Escaped)

	t, err := template.New(models.ReportHTMLFileName).Funcs(funcMap).Parse(reportTmpl)
	if err != nil {
		return fmt.Errorf("Error while parse template: %w ", err)
	}

	file, err := createOrTruncateReportFile(models.ReportHTMLFileName)
	if err != nil {
		return fmt.Errorf("Error while open/create .html report file from template: %w ", err)
	}
	defer closeReportFile(file, models.ReportHTMLFileName)

	data := struct {
		Stats          models.Stats
		GroupedMutants map[string][]models.Mutant
	}{
		Stats:          report.Stats,
		GroupedMutants: groupedMutants,
	}

	err = t.Execute(file, data)
	if err != nil {
		return fmt.Errorf("Error while execute template for .html report: %w ", err)
	}

	return nil
}

// MakeJSONReport is a function for creating json report, which is based on models.Report
func MakeJSONReport(report models.Report) error {
	jsonContent, err := json.Marshal(report)
	if err != nil {
		return err
	}

	file, err := createOrTruncateReportFile(models.ReportFileName)
	if err != nil {
		return fmt.Errorf("Error while open/create .json report file from template: %w ", err)
	}
	defer closeReportFile(file, models.ReportFileName)

	if file == nil {
		return errors.New("cannot create file for .json report")
	}

	_, err = file.WriteString(string(jsonContent))
	if err != nil {
		return err
	}

	return nil
}

// MakeSummaryJSONReport writes a compact stats-only JSON to mutago-summary.json.
// Useful for badge generation and CI dashboards that don't need per-mutant detail.
func MakeSummaryJSONReport(stats models.Stats) error {
	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	file, err := createOrTruncateReportFile(models.ReportSummaryJSONFileName)
	if err != nil {
		return fmt.Errorf("Error while open/create summary JSON report file: %w", err)
	}
	defer closeReportFile(file, models.ReportSummaryJSONFileName)

	if file == nil {
		return errors.New("cannot create file for summary JSON report")
	}

	_, err = file.WriteString(string(data))
	return err
}

func createOrTruncateReportFile(filename string) (*os.File, error) {
	return os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func closeReportFile(file *os.File, filename string) {
	if err := file.Close(); err != nil {
		fmt.Printf("Error while closing %s: %v\n", filename, err)
	}
}

// AgenticMutant describes one escaped mutant for LLM consumption.
type AgenticMutant struct {
	ID               string   `json:"id"`
	File             string   `json:"file"`
	Line             int64    `json:"line"`
	Mutator          string   `json:"mutator"`
	Description      string   `json:"description,omitempty"`
	KillHint         string   `json:"kill_hint,omitempty"`
	Diff             string   `json:"diff"`
	ContextStartLine int      `json:"context_start_line,omitempty"`
	ContextLines     []string `json:"context_lines,omitempty"`
	TestFiles        []string `json:"test_files,omitempty"`
}

const agenticReminder = "A mutant is an example of how this code could be wrong — it's not a script for the test. Don't assert on the mutant directly. Instead ask: if this code were buggy, what would a caller of the public API observe go wrong? Write a test for that."

type agenticReport struct {
	GeneratedAt  string          `json:"generated_at"`
	Msi          float64         `json:"msi"`
	EscapedCount int             `json:"escaped_count"`
	Reminder     string          `json:"reminder"`
	Mutants      []AgenticMutant `json:"mutants"`
}

// generateInstanceDescription produces a diff-aware description for one escaped
// mutant. For simple one-line changes it returns "Changes: <from> → <to>".
// For multi-line or unparseable diffs it falls back to the static per-mutator
// description.
func generateInstanceDescription(mutatorName, diff string) string {
	fromLines, toLines := diffChangedLines(diff)
	if desc, ok := singleLineChangeDesc(fromLines, toLines); ok {
		return desc
	}
	if desc, ok := mutatorDescriptions[mutatorName]; ok {
		return desc
	}
	return ""
}

// diffChangedLines splits a unified diff into its removed (from) and added (to)
// content lines, ignoring file and hunk headers.
func diffChangedLines(diff string) (fromLines, toLines []string) {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "@@ ") {
			continue
		}
		if strings.HasPrefix(line, "-") {
			fromLines = append(fromLines, strings.TrimPrefix(line, "-"))
			continue
		}
		if strings.HasPrefix(line, "+") {
			toLines = append(toLines, strings.TrimPrefix(line, "+"))
		}
	}
	return fromLines, toLines
}

// singleLineChangeDesc returns a "Changes: from → to" description when the diff
// is a single-line replacement, reporting ok=false otherwise.
func singleLineChangeDesc(fromLines, toLines []string) (string, bool) {
	if len(fromLines) != 1 || len(toLines) != 1 {
		return "", false
	}
	from := strings.TrimSpace(fromLines[0])
	to := strings.TrimSpace(toLines[0])
	if from == "" || to == "" || from == to {
		return "", false
	}
	return fmt.Sprintf("Changes: `%s` → `%s`", from, to), true
}

// MakeAgenticJSONReport writes mutago-agentic.json with enriched escaped-mutant
// data designed for LLM consumption: stable IDs, context lines, test file paths,
// mutator descriptions, and heuristic test-writing hints.
func MakeAgenticJSONReport(report models.Report, moduleRoot string) error {
	mutants := make([]AgenticMutant, 0, len(report.Escaped))
	for _, m := range report.Escaped {
		relFile := toRelPath(m.Mutator.OriginalFilePath, moduleRoot)
		id := baseline.MutantID(relFile, m.Mutator.MutatorName, m.Diff)
		const contextRadius = 3
		ctxLines, ctxStart := extractContextLines(m.Mutator.OriginalSourceCode, int(m.Mutator.OriginalStartLine), contextRadius)
		mutants = append(mutants, AgenticMutant{
			ID:               id,
			File:             relFile,
			Line:             m.Mutator.OriginalStartLine,
			Mutator:          m.Mutator.MutatorName,
			Description:      generateInstanceDescription(m.Mutator.MutatorName, m.Diff),
			KillHint:         killHints[m.Mutator.MutatorName],
			Diff:             m.Diff,
			ContextStartLine: ctxStart,
			ContextLines:     ctxLines,
			TestFiles:        findTestFiles(filepath.Dir(m.Mutator.OriginalFilePath), moduleRoot),
		})
	}

	doc := agenticReport{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Msi:          report.Stats.Msi,
		EscapedCount: len(report.Escaped),
		Reminder:     agenticReminder,
		Mutants:      mutants,
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	file, err := createOrTruncateReportFile(models.ReportAgenticJSONFileName)
	if err != nil {
		return fmt.Errorf("Error while open/create agentic JSON report file: %w", err)
	}
	defer closeReportFile(file, models.ReportAgenticJSONFileName)

	if file == nil {
		return errors.New("cannot create file for agentic JSON report")
	}

	_, err = file.WriteString(string(data))
	return err
}

// extractContextLines returns up to radius lines before and after line (1-based)
// from the given source string, along with the 1-based line number of the first
// returned line (context_start_line).
func extractContextLines(source string, line, radius int) ([]string, int) {
	if source == "" || line <= 0 {
		return nil, 0
	}
	lines := strings.Split(source, "\n")
	start := max(line-radius-1, 0)
	start = min(start, len(lines))
	end := min(line+radius-1, len(lines)-1)
	end = max(end, start-1)
	return lines[start : end+1], start + 1
}

func toRelPath(absOrRel, moduleRoot string) string {
	rel, err := filepath.Rel(moduleRoot, absOrRel)
	if err != nil {
		return filepath.ToSlash(absOrRel)
	}
	return filepath.ToSlash(rel)
}

// findTestFiles returns relative paths to *_test.go files in dir.
func findTestFiles(dir, moduleRoot string) []string {
	matches, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	result := make([]string, 0, len(matches))
	for _, f := range matches {
		rel, err := filepath.Rel(moduleRoot, f)
		if err != nil {
			result = append(result, filepath.ToSlash(f))
			continue
		}
		result = append(result, filepath.ToSlash(rel))
	}
	return result
}

// gitLabIssue is one entry in the GitLab Code Quality JSON array.
type gitLabIssue struct {
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Severity    string         `json:"severity"`
	Fingerprint string         `json:"fingerprint"`
	Location    gitLabLocation `json:"location"`
}

type gitLabLocation struct {
	Path  string      `json:"path"`
	Lines gitLabLines `json:"lines"`
}

type gitLabLines struct {
	Begin int `json:"begin"`
}

// MakeGitLabReport writes mutago-gitlab.json in GitLab Code Quality
// format. Each escaped mutant becomes one issue entry. GitLab picks this up
// automatically when the artifact path is configured in .gitlab-ci.yml.
func MakeGitLabReport(report models.Report, moduleRoot string) error {
	issues := make([]gitLabIssue, 0, len(report.Escaped))
	for _, m := range report.Escaped {
		relFile := toRelPath(m.Mutator.OriginalFilePath, moduleRoot)
		id := baseline.MutantID(relFile, m.Mutator.MutatorName, m.Diff)
		desc := fmt.Sprintf("Escaped mutant (%s) at %s:%d — no test kills this mutation",
			m.Mutator.MutatorName, relFile, m.Mutator.OriginalStartLine)
		issues = append(issues, gitLabIssue{
			Type:        "issue",
			Description: desc,
			Severity:    "minor",
			Fingerprint: id,
			Location: gitLabLocation{
				Path:  relFile,
				Lines: gitLabLines{Begin: int(m.Mutator.OriginalStartLine)},
			},
		})
	}

	data, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		return err
	}

	file, err := createOrTruncateReportFile(models.ReportGitLabJSONFileName)
	if err != nil {
		return fmt.Errorf("Error while open/create GitLab report file: %w", err)
	}
	defer closeReportFile(file, models.ReportGitLabJSONFileName)

	if file == nil {
		return errors.New("cannot create file for GitLab report")
	}

	_, err = file.WriteString(string(data))
	return err
}

func groupEscapedMutants(escaped []models.Mutant) map[string][]models.Mutant {
	if len(escaped) == 0 {
		return make(map[string][]models.Mutant)
	}

	mutantCount := make(map[string]int)
	for _, mutant := range escaped {
		filePath := mutant.Mutator.OriginalFilePath
		mutantCount[filePath]++
	}

	groupedMutants := make(map[string][]models.Mutant, len(mutantCount))
	for filePath, count := range mutantCount {
		groupedMutants[filePath] = make([]models.Mutant, 0, count)
	}

	for _, mutant := range escaped {
		filePath := mutant.Mutator.OriginalFilePath
		groupedMutants[filePath] = append(groupedMutants[filePath], mutant)
	}

	return groupedMutants
}
