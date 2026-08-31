package importing

import (
	"os"
	"sort"
	"testing"
)

// Regression for issue #76: matchPackages("std") walked gorootSrcPkg =
// $GOROOT/src/pkg, a directory removed in Go 1.4 (stdlib now lives under
// $GOROOT/src). The walk found no stdlib packages, so "std" silently returned
// only cmd/* packages. The fix points the std walk at $GOROOT/src.
func TestStdPatternFindsStdlibPackages(t *testing.T) {
	// The std source root used by walkSrcPackages must actually exist.
	if fi, err := os.Stat(gorootSrc); err != nil || !fi.IsDir() {
		t.Fatalf("BUG: std source root %q does not exist (src/pkg was removed in Go 1.4; stdlib is under $GOROOT/src): %v", gorootSrc, err)
	}

	pkgs := matchPackages("std")
	if len(pkgs) == 0 {
		t.Fatal("BUG: matchPackages(\"std\") returned no packages; stdlib resolution is broken")
	}

	// A handful of stable, always-present stdlib packages.
	want := []string{"fmt", "strings", "os"}
	have := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		have[p] = true
	}
	var missing []string
	for _, w := range want {
		if !have[w] {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		// Show a small sample for diagnosis.
		sample := pkgs
		if len(sample) > 20 {
			sample = sample[:20]
		}
		t.Errorf("BUG: matchPackages(\"std\") missing expected stdlib packages %v; sample of returned: %v", missing, sample)
	}
}
