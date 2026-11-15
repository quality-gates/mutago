package coverage

import (
	"reflect"
	"testing"
)

func TestPathResolutionRepeatedHitsAndMisses(t *testing.T) {
	p := &Profile{coveredLines: map[string]map[int]bool{"a.go": {1: true}, "pkg/a.go": {2: true}}}
	per := &PerTestProfile{data: map[string]map[int][]string{"a.go": {1: {"TestA"}}, "pkg/a.go": {2: {"TestB"}}}}
	for i := 0; i < 3; i++ {
		if !p.IsCovered("/repo/pkg/a.go", 2) || p.IsCovered("/repo/pkg/a.go", 1) || p.IsCovered("/repo/missing.go", 1) {
			t.Fatal("coverage hit or miss changed")
		}
		if !reflect.DeepEqual(per.CoveringTests("/repo/pkg/a.go", 2), []string{"TestB"}) || per.CoveringTests("/repo/missing.go", 1) != nil {
			t.Fatal("per-test hit or miss changed")
		}
	}
	p.coveredLines["missing.go"] = map[int]bool{1: true}
	per.data["missing.go"] = map[int][]string{1: {"TestLate"}}
	if p.IsCovered("/repo/missing.go", 1) || per.CoveringTests("/repo/missing.go", 1) != nil {
		t.Fatal("cached miss changed")
	}
}
