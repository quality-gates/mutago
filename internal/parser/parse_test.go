package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/quality-gates/mutago/v2/internal/annotation"
	"github.com/quality-gates/mutago/v2/internal/filter"
)

func TestParseAndTypeCheckFileTypeCheckWholePackage(t *testing.T) {
	annotationProcessor := annotation.NewProcessor()
	skipFilterProcessor := filter.NewSkipMakeArgsFilter()

	collectors := []filter.NodeCollector{
		annotationProcessor,
		skipFilterProcessor,
	}
	_, _, _, _, err := ParseAndTypeCheckFile("../../astutil/create.go", collectors)
	assert.Nil(t, err)
}
