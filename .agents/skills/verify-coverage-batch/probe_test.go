package coverage

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestVerifyCoverageBatch(t *testing.T) {
	for count := 0; count <= 3; count++ {
		for mask := 0; mask < 1<<count; mask++ {
			tc := scriptedToolchain{runs: make(map[string]string)}
			var batch profileBatch
			binaryPath := filepath.Join(t.TempDir(), "tests")
			for i := 0; i < count; i++ {
				name := fmt.Sprintf("Test%d", i)
				batch.jobs = append(batch.jobs, perTestJob{name: name, binaryPath: binaryPath})
				tc.runs[name] = fmt.Sprintf("printf 'mode: set\\nx.go:%d.1,%d.2 1 1\\n' > \"$1\"", i+1, i+1)
				if mask&(1<<i) != 0 {
					tc.runs[name] = "exit 0"
				}
			}
			got, err := batch.collect(tc, 2, "example.com/m")
			if mask != 0 {
				if err == nil || got != nil {
					t.Fatalf("count=%d missing=%03b: accepted partial profile", count, mask)
				}
				continue
			}
			if err != nil || got == nil {
				t.Fatalf("count=%d: complete profiles rejected: %v", count, err)
			}
			for i := 0; i < count; i++ {
				names := got.CoveringTestsRelative("x.go", i+1)
				if len(names) != 1 || names[0] != fmt.Sprintf("Test%d", i) {
					t.Fatalf("count=%d line=%d: got %v", count, i+1, names)
				}
			}
		}
	}
}
