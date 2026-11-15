package engine

import (
	"bytes"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/internal/models"
)

func TestGenerateRemembersFailureAfterSuccessfulWrite(t *testing.T) {
	var output, errors bytes.Buffer
	sink := &generateSink{stderr: &errors}
	fc := &fileContext{mutatedFile: filepath.Join(t.TempDir(), "mutant"), originalSource: []byte("x")}
	sink.admit(fc, mutago.PositionedMutation{}, 1, preparedMutation{edit: mutationEdit{start: 2, end: 2}})
	sink.admit(fc, mutago.PositionedMutation{}, 2, preparedMutation{edit: mutationEdit{start: 0, end: 1, replacement: []byte("y")}})
	run := &mutationRun{opts: models.NewOptions(), stdout: &output, stderr: &errors, runMutantIDFound: &atomic.Bool{}}
	if code := sink.finish(run, baseline.Capture(nil)); code != returnError {
		t.Fatalf("failure was forgotten: exit %d", code)
	}
}
