package models_test

import (
	"encoding/json"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
	"github.com/stretchr/testify/require"
)

func TestReportLocationViews(t *testing.T) {
	for _, tc := range []struct {
		name     string
		document string
		physical models.Coordinate
		coverage models.Coordinate
	}{
		{"legacy", `{"originalFilePath":"calc.go","originalStartLine":200}`, models.Coordinate{Filename: "calc.go", Line: 200}, models.Coordinate{Filename: "calc.go", Line: 200}},
		{"paired", `{"originalFilePath":"calc.go","originalStartLine":200,"location":{"physical":{"filename":"calc.go","line":5},"coverage":{"filename":"virtual.go","line":200}}}`, models.Coordinate{Filename: "calc.go", Line: 5}, models.Coordinate{Filename: "virtual.go", Line: 200}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var m models.Mutator
			require.NoError(t, json.Unmarshal([]byte(tc.document), &m))
			require.Equal(t, tc.physical, m.SourceLocation())
			require.Equal(t, tc.coverage, m.CoverageLocation())
			data, err := json.Marshal(m)
			require.NoError(t, err)
			require.JSONEq(t, tc.document[:len(tc.document)-1]+`,"mutatorName":""}`, string(data))
		})
	}
}
