package baseline

import (
	"github.com/quality-gates/mutago/v2/internal/models"
	"testing"
)

func TestVerificationBaseline(t *testing.T) {
	for present := 0; present < 2; present++ {
		for known := 0; known < 8; known++ {
			var file *File
			if present == 1 {
				file = &File{}
				for i := 0; i < 3; i++ {
					if known&(1<<i) != 0 {
						file.Mutants = append(file.Mutants, Entry{ID: string(rune('a' + i))})
					}
				}
			}
			snap := Capture(file)
			for escaped := 0; escaped < 8; escaped++ {
				var input []models.Mutant
				for i := 0; i < 3; i++ {
					if escaped&(1<<i) != 0 {
						input = append(input, models.Mutant{ID: string(rune('a' + i))})
					}
				}
				got := snap.Compare(input)
				if got.BaselinePresent != (present == 1) {
					t.Fatal("incorrect baseline presence")
				}
				actual := map[string]bool{}
				for _, m := range got.Mutants {
					actual[m.ID] = true
				}
				for i := 0; i < 3; i++ {
					want := escaped&(1<<i) != 0 && (present == 0 || known&(1<<i) == 0)
					if actual[string(rune('a'+i))] != want {
						t.Fatalf("incorrect escapes: present=%d known=%d escaped=%d", present, known, escaped)
					}
				}
			}
		}
	}
}
