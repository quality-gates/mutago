package coverage

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzParseProfile exercises the Go coverage-profile parser with arbitrary
// profile bytes. It must never panic. parseLine performs index slicing,
// integer parsing and range expansion on untrusted-shaped input.
func FuzzParseProfile(f *testing.F) {
	seeds := []string{
		"mode: set\n",
		"mode: set\ngithub.com/acme/app/pkg/foo.go:5.1,10.3 2 1\n",
		"github.com/acme/app/pkg/foo.go:5.1,10.3 2 0\n",
		"foo.go:1.1,1.1 1 1\n",
		"foo.go:1.1,1.1 1 x\n",
		"foo.go:bad,worse 1 1\n",
		"foo.go:\n",
		":\n",
		"no colon here\n",
		"foo.go:1.1 1 1\n",
		"foo.go:1.1,2.2 extra fields here 1\n",
		"a:b:c:1.1,2.2 1 1\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, content string) {
		dir := t.TempDir()
		path := filepath.Join(dir, "coverage.out")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Skip()
		}
		prof, err := ParseProfile(path, "github.com/acme/app")
		if err != nil {
			return
		}
		// IsCovered must not panic on whatever profile we built.
		_ = prof.IsCovered("/some/abs/pkg/foo.go", 5)
	})
}
