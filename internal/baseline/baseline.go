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
func MutantID(relFile, mutatorName, diff string) string {
	var removed, added strings.Builder
	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
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

// NewEscapes returns mutants from escaped that are not recorded in this baseline.
// When f is nil (no baseline loaded), all mutants are considered new.
func (f *File) NewEscapes(escaped []models.Mutant, moduleRoot string) []models.Mutant {
	if f == nil {
		return escaped
	}
	knownIDs := f.IDSet()
	var result []models.Mutant
	for _, m := range escaped {
		relFile := toRelPath(m.Mutator.OriginalFilePath, moduleRoot)
		id := MutantID(relFile, m.Mutator.MutatorName, m.Diff)
		if _, known := knownIDs[id]; !known {
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
		relFile := toRelPath(m.Mutator.OriginalFilePath, moduleRoot)
		entries = append(entries, Entry{
			ID:      MutantID(relFile, m.Mutator.MutatorName, m.Diff),
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

func toRelPath(absOrRel, moduleRoot string) string {
	rel, err := filepath.Rel(moduleRoot, absOrRel)
	if err != nil {
		return filepath.ToSlash(absOrRel)
	}
	return filepath.ToSlash(rel)
}
