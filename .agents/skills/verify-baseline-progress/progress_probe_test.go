package console

import (
	"bytes"
	"github.com/quality-gates/mutago/v2/internal/models"
	"testing"
)

func TestVerificationProgress(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		for mask := 0; mask < 64; mask++ {
			var out bytes.Buffer
			r := NewTerminalReporter(&models.Options{}, &bytes.Buffer{}, &out, terminal)
			drawn := false
			for i := 0; i < 6; i++ {
				before := out.Len()
				if mask&(1<<i) == 0 {
					r.OnProgress(models.Stats{})
					if terminal {
						drawn = true
					}
					if (out.Len() > before) != terminal {
						t.Fatalf("draw mismatch terminal=%v", terminal)
					}
				} else {
					r.OnSummary(nil)
					if (out.Len() > before) != drawn {
						t.Fatalf("clear mismatch terminal=%v mask=%d step=%d", terminal, mask, i)
					}
					drawn = false
				}
			}
		}
	}
}
