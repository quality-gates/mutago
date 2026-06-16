package gitdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestParseChangedLines_StaleBranchExcludesTargetChanges guards against a bug
// where a feature branch that is behind its target picked up changes that
// landed on the target branch after the feature branched off.
//
// ParseChangedLines runs a two-dot `git diff <base>`, which compares the tip of
// base directly against the working tree. The correct, PR-equivalent comparison
// is a three-dot `git diff <base>...HEAD`, which diffs from the merge-base.
//
// Scenario:
//   - main:    A=1 B=2   C=3   (initial)
//   - main:    A=1 B=222 C=3   (B changed on main, AFTER feature branched)
//   - feature: A=1 B=2   C=333 (only C changed in the PR)
//
// The developer's PR only touches line 5 (func C). The bug makes mutago also
// consider line 4 (func B) "changed", because two-dot diff attributes main's
// own commit to the feature branch.
func TestParseChangedLines_StaleBranchExcludesTargetChanges(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(b int, c int) {
		t.Helper()
		src := "package app\n\n" +
			"func A() int { return 1 }\n" +
			"func B() int { return " + itoa(b) + " }\n" +
			"func C() int { return " + itoa(c) + " }\n"
		if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "t@t.com")
	run("config", "user.name", "t")

	// initial commit on main: B=2 C=3
	write(2, 3)
	run("add", "app.go")
	run("commit", "-qm", "initial")

	// branch off for feature work (feature's merge-base is this commit)
	run("switch", "-qc", "feature")

	// main advances: B changes to 222 (NOT part of the feature PR)
	run("switch", "-q", "main")
	write(222, 3)
	run("commit", "-qam", "main: change B")

	// feature changes only C (the sole change in the PR)
	run("switch", "-q", "feature")
	write(2, 333)
	run("commit", "-qam", "feature: change C")

	// Run the tool's diff from the feature branch's working tree.
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	cl, err := ParseChangedLines("main")
	if err != nil {
		t.Fatal(err)
	}

	ranges := cl["app.go"]
	t.Logf("ParseChangedLines(\"main\") => %v", ranges)

	changed := func(line int) bool {
		for _, r := range ranges {
			if line >= r.Start && line <= r.End {
				return true
			}
		}
		return false
	}

	// The PR only touched line 5 (func C). That must be flagged.
	if !changed(5) {
		t.Errorf("expected line 5 (func C, the real PR change) to be flagged changed")
	}

	// Line 4 (func B) was changed on main AFTER the feature branched, not in the
	// PR. A correct merge-base (three-dot) comparison must NOT flag it. With the
	// buggy two-dot `git diff main` this assertion fails, because main's own
	// commit gets attributed to the feature branch.
	if changed(4) {
		t.Errorf("line 4 (func B) was changed on main, not in the feature PR, and "+
			"must not be flagged as changed. ranges=%v", ranges)
	}

	// Uncommitted working-tree changes must still be reported. Diffing against
	// the merge-base commit (rather than `base...HEAD`) preserves this: edit
	// func A on the working tree without committing and confirm line 3 appears.
	write(2, 333) // keep C=333, now also change A below via a manual rewrite
	src := "package app\n\n" +
		"func A() int { return 11 }\n" + // line 3 changed, uncommitted
		"func B() int { return 2 }\n" +
		"func C() int { return 333 }\n"
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cl2, err := ParseChangedLines("main")
	if err != nil {
		t.Fatal(err)
	}
	ranges2 := cl2["app.go"]
	t.Logf("after uncommitted edit, ParseChangedLines(\"main\") => %v", ranges2)
	changed2 := func(line int) bool {
		for _, r := range ranges2 {
			if line >= r.Start && line <= r.End {
				return true
			}
		}
		return false
	}
	if !changed2(3) {
		t.Errorf("uncommitted change to line 3 (func A) must be reported. ranges=%v", ranges2)
	}
	if changed2(4) {
		t.Errorf("line 4 (func B, changed on main) must still be excluded. ranges=%v", ranges2)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
