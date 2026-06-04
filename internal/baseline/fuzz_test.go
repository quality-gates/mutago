package baseline

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzMutantID checks that the stable-ID hash never panics on arbitrary diff
// text and always returns a fixed-width hex string.
func FuzzMutantID(f *testing.F) {
	seeds := []string{
		"",
		"--- a\n+++ b\n-foo\n+bar\n",
		"-\n+\n",
		"+++\n---\n",
		"no markers at all",
		"\n\n\n",
	}
	for _, s := range seeds {
		f.Add("file.go", "arithmetic", s)
	}

	f.Fuzz(func(t *testing.T, relFile, mutatorName, diff string) {
		id := MutantID(relFile, mutatorName, diff)
		if len(id) != 32 {
			t.Fatalf("MutantID returned %d-char id %q (want 32)", len(id), id)
		}
	})
}

// FuzzLoad exercises the JSON baseline loader with arbitrary file contents.
// It must never panic; malformed JSON should surface as an error, not a crash.
func FuzzLoad(f *testing.F) {
	seeds := []string{
		`{"version":1,"mutants":[]}`,
		`{"version":1,"mutants":[{"id":"x","file":"a.go","mutator":"m","line":3}]}`,
		`{`,
		``,
		`null`,
		`{"mutants":null}`,
		`[]`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, content string) {
		dir := t.TempDir()
		path := filepath.Join(dir, "baseline.json")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Skip()
		}
		fl, err := Load(path)
		if err != nil {
			return
		}
		if fl != nil {
			// IDSet and NewEscapes must not panic on the parsed file.
			_ = fl.IDSet()
			_ = fl.NewEscapes(nil, "/module/root")
		}
	})
}
