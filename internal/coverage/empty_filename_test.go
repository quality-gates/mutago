package coverage

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverageIgnoresEmptyFilename(t *testing.T) {
	for _, filename := range []string{"", modulePath + "/"} {
		t.Run("filename="+filename, func(t *testing.T) {
			path := writeTmpProfile(t, "mode: set\n"+filename+":1.1,3.3 1 1\n"+
				modulePath+"/pkg/known.go:5.1,5.3 1 1\n")
			profile, err := ParseProfile(path, modulePath)
			require.NoError(t, err)
			results := make(chan perTestResult, 1)
			results <- perTestResult{name: "TestKnown", prof: profile}
			perTest := mergePerTestProfiles(results, 1)

			for _, file := range []string{"unknown.go", "other/missing.go"} {
				absFile := filepath.Join(t.TempDir(), filepath.FromSlash(file))
				for lookup := range 2 {
					assert.False(t, profile.IsCovered(absFile, 1), "unrelated file must remain uncovered, lookup %d", lookup)
					assert.Nil(t, perTest.CoveringTests(absFile, 1), "unrelated file must have no covering tests, lookup %d", lookup)
				}
			}
			knownFile := filepath.Join(t.TempDir(), "pkg", "known.go")
			assert.True(t, profile.IsCovered(knownFile, 5))
			assert.Equal(t, []string{"TestKnown"}, perTest.CoveringTests(knownFile, 5))
		})
	}
}
