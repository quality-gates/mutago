package engine

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
)

func TestVerifyCoverageFallback(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		prof, err := perTestUnavailable(ctx, &models.Options{}, "example.com/pkg", errors.New("preparation failed"))
		cancel()
		if prof != nil {
			t.Fatal("failure returned a profile")
		}
		if cancelled && !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
		if !cancelled && err != nil {
			t.Fatalf("ordinary failure should fall back: %v", err)
		}
	}
}

func TestVerifyStableExecFallback(t *testing.T) {
	dir := t.TempDir()
	wrapper, err := writeStableExecWrapper(dir)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "source")
	if err := os.WriteFile(src, []byte("#!/bin/sh\nprintf 'original:%s' \"$1\"\nexit 9\n"), 0755); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, stableDir := range []string{"", blocked} {
		cmd := exec.Command(wrapper, src, "arg")
		cmd.Env = append(os.Environ(), "MUTAGO_STABLE_DIR="+stableDir)
		output, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 9 || !strings.Contains(string(output), "original:arg") {
			t.Fatalf("fallback failed for %q: output=%q error=%v", stableDir, output, err)
		}
	}
}
