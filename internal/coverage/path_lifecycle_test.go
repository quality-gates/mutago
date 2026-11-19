package coverage

import (
	"reflect"
	"testing"
)

func TestPathResolutionRepeatedHitsAndMisses(t *testing.T) {
	p := &Profile{coveredRanges: map[string][]lineRange{"a.go": {{start: 1, end: 1}}, "pkg/a.go": {{start: 2, end: 2}}}}
	per := &PerTestProfile{data: map[string][]namedRange{"a.go": {{start: 1, end: 1, test: "TestA"}}, "pkg/a.go": {{start: 2, end: 2, test: "TestB"}}}}
	for i := 0; i < 3; i++ {
		if !p.IsCovered("/repo/pkg/a.go", 2) || p.IsCovered("/repo/pkg/a.go", 1) || p.IsCovered("/repo/missing.go", 1) {
			t.Fatal("coverage hit or miss changed")
		}
		if !reflect.DeepEqual(per.CoveringTests("/repo/pkg/a.go", 2), []string{"TestB"}) || per.CoveringTests("/repo/missing.go", 1) != nil {
			t.Fatal("per-test hit or miss changed")
		}
	}
	p.coveredRanges["missing.go"] = []lineRange{{start: 1, end: 1}}
	per.data["missing.go"] = []namedRange{{start: 1, end: 1, test: "TestLate"}}
	if p.IsCovered("/repo/missing.go", 1) || per.CoveringTests("/repo/missing.go", 1) != nil {
		t.Fatal("cached miss changed")
	}
}
