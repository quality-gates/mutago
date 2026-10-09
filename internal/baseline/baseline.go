// Package baseline tracks known-surviving mutants so CI fails only on new
// regressions, not on pre-existing escapes that the team has accepted.
//
// Usage:
//
//	First run:  --update-baseline  (writes current escaped set, exits 0)
//	CI:         baseline file committed; new runs fail only on NEW escapes
package baseline

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/quality-gates/mutago/v2/internal/models"
)

// File is the on-disk baseline format.
type File struct {
	Version int     `json:"version"`
	Mutants []Entry `json:"mutants"`
}

// Entry describes one known-surviving mutant.
type Entry struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Mutator string `json:"mutator"`
	Line    int64  `json:"line"`
}

// MutantID returns a stable identifier for a mutant.
// It hashes the relative file path, mutator name, and the actual changed
// lines from the diff — deliberately excluding line numbers so the ID
// survives refactors that only shift surrounding code.
// It is MutantIDAt with occurrence 0.
func MutantID(relFile, mutatorName, diff string) string {
	return MutantIDAt(relFile, mutatorName, diff, 0)
}

// MutantIDAt returns the stable identifier of the nth mutant (from 0, in source
// order) whose file, mutator and changed lines all match. Occurrence 0 keeps
// the MutantID hash, so baselines written before same-text mutants were told
// apart stay valid for every mutant except the later members of such a group.
func MutantIDAt(relFile, mutatorName, diff string, occurrence int) string {
	var removed, added strings.Builder
	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ "):
			// skip diff header lines
		case strings.HasPrefix(line, "-"):
			removed.WriteString(strings.TrimPrefix(line, "-"))
			removed.WriteByte('\n')
		case strings.HasPrefix(line, "+"):
			added.WriteString(strings.TrimPrefix(line, "+"))
			added.WriteByte('\n')
		}
	}
	key := relFile + "\x00" + mutatorName + "\x00" + removed.String() + "\x00" + added.String()
	if occurrence > 0 {
		key += "\x00" + strconv.Itoa(occurrence)
	}
	h := md5.Sum([]byte(key))
	return fmt.Sprintf("%x", h)
}

// Load reads the baseline file.
// Returns (nil, nil) when the file does not exist — callers treat this as
// "no baseline active" and skip the check entirely.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read baseline %q: %w", path, err)
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse baseline %q: %w", path, err)
	}
	return &f, nil
}

// IDSet returns the set of known mutant IDs for fast O(1) lookup.
func (f *File) IDSet() map[string]struct{} {
	s := make(map[string]struct{}, len(f.Mutants))
	for _, m := range f.Mutants {
		s[m.ID] = struct{}{}
	}
	return s
}

// NewEscapes returns mutants from escaped whose ID is not recorded in this
// baseline. When f is nil (no baseline loaded), all mutants are considered new.
func (f *File) NewEscapes(escaped []models.Mutant) []models.Mutant {
	if f == nil {
		return escaped
	}
	knownIDs := f.IDSet()
	var result []models.Mutant
	for _, m := range escaped {
		if _, known := knownIDs[m.ID]; !known {
			result = append(result, m)
		}
	}
	return result
}

// Write serialises the escaped mutants to the baseline file.
// moduleRoot is used to make file paths relative and portable.
func Write(path string, escaped []models.Mutant, moduleRoot string) error {
	entries := make([]Entry, 0, len(escaped))
	for _, m := range escaped {
		relFile := RelPath(m.Mutator.OriginalFilePath, moduleRoot)
		entries = append(entries, Entry{
			ID:      m.ID,
			File:    relFile,
			Mutator: m.Mutator.MutatorName,
			Line:    m.Mutator.OriginalStartLine,
		})
	}
	f := File{Version: 1, Mutants: entries}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// RelPath returns the canonical identity path of a source file: slash-separated
// and relative to moduleRoot. It is the single owner of the path that
// MutantID hashes, so every spelling of one file ("./a/b.go", "a/b.go", an
// absolute path, or "b.go" from inside a/) yields the same mutant ID.
// A relative path is resolved against the process working directory first,
// because that is the directory a command-line target refers to.
func RelPath(path, moduleRoot string) string {
	if filepath.IsAbs(moduleRoot) && !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	rel, err := filepath.Rel(moduleRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
