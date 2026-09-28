package engine

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestGoTestInvocationArgs(t *testing.T) {
	const pkg = "example.com/m/pkg"
	tests := []struct {
		name string
		inv  goTestInvocation
		want []string
	}{
		{
			name: "baseline",
			inv:  goTestInvocation{kind: baselineRun, target: pkg, timeoutSeconds: 10},
			want: []string{"test", "-timeout", "10s", "-vet=off", pkg},
		},
		{
			name: "baseline recursive with user flags",
			inv:  goTestInvocation{kind: baselineRun, target: pkg, recursive: true, timeoutSeconds: 10, testFlags: []string{"-count=1", "-vet=atomic"}},
			want: []string{"test", "-timeout", "10s", "-count=1", "-vet=atomic", pkg + "/..."},
		},
		{
			name: "coverage",
			inv:  goTestInvocation{kind: coverageRun, target: pkg, timeoutSeconds: 20, profilePath: "/tmp/c.out"},
			want: []string{"test", "-coverprofile=/tmp/c.out", "-timeout", "20s", "-vet=off", pkg},
		},
		{
			name: "coverage recursive attributes subpackage tests to the target",
			inv:  goTestInvocation{kind: coverageRun, target: pkg, recursive: true, timeoutSeconds: 20, profilePath: "/tmp/c.out"},
			want: []string{"test", "-coverprofile=/tmp/c.out", "-coverpkg=" + pkg, "-timeout", "20s", "-vet=off", pkg + "/..."},
		},
		{
			name: "coverage with user vet flag",
			inv:  goTestInvocation{kind: coverageRun, target: pkg, timeoutSeconds: 20, profilePath: "/tmp/c.out", testFlags: []string{"--vet=off"}},
			want: []string{"test", "-coverprofile=/tmp/c.out", "-timeout", "20s", "--vet=off", pkg},
		},
		{
			name: "mutant",
			inv:  goTestInvocation{kind: mutantRun, target: pkg, timeoutSeconds: 5, overlay: "o.json"},
			want: []string{"test", "-overlay=o.json", "-timeout", "5s", "-vet=off", "-failfast", pkg},
		},
		{
			name: "mutant recursive with user flags and run filter",
			inv:  goTestInvocation{kind: mutantRun, target: pkg, recursive: true, timeoutSeconds: 5, overlay: "o.json", testFlags: []string{"-vet=atomic", "-failfast=false"}, runFilter: "^(TestA)$"},
			want: []string{"test", "-overlay=o.json", "-timeout", "5s", "-vet=atomic", "-failfast=false", "-run", "^(TestA)$", pkg + "/..."},
		},
		{
			name: "baseline with user flags",
			inv:  goTestInvocation{kind: baselineRun, target: pkg, timeoutSeconds: 10, testFlags: []string{"-short"}},
			want: []string{"test", "-timeout", "10s", "-short", "-vet=off", pkg},
		},
		{
			name: "coverage recursive with user flags",
			inv:  goTestInvocation{kind: coverageRun, target: pkg, recursive: true, timeoutSeconds: 20, profilePath: "/tmp/c.out", testFlags: []string{"-short"}},
			want: []string{"test", "-coverprofile=/tmp/c.out", "-coverpkg=" + pkg, "-timeout", "20s", "-short", "-vet=off", pkg + "/..."},
		},
		{
			name: "mutant recursive",
			inv:  goTestInvocation{kind: mutantRun, target: pkg, recursive: true, timeoutSeconds: 5, overlay: "o.json"},
			want: []string{"test", "-overlay=o.json", "-timeout", "5s", "-vet=off", "-failfast", pkg + "/..."},
		},
		{
			name: "mutant with run filter",
			inv:  goTestInvocation{kind: mutantRun, target: pkg, timeoutSeconds: 5, overlay: "o.json", runFilter: "^(TestA)$"},
			want: []string{"test", "-overlay=o.json", "-timeout", "5s", "-vet=off", "-failfast", "-run", "^(TestA)$", pkg},
		},
		{
			name: "run filter is ignored for coverage runs",
			inv:  goTestInvocation{kind: coverageRun, target: pkg, timeoutSeconds: 20, profilePath: "/tmp/c.out", runFilter: "^(TestA)$"},
			want: []string{"test", "-coverprofile=/tmp/c.out", "-timeout", "20s", "-vet=off", pkg},
		},
		{
			name: "run filter is ignored for baseline runs",
			inv:  goTestInvocation{kind: baselineRun, target: pkg, timeoutSeconds: 10, runFilter: "^(TestA)$"},
			want: []string{"test", "-timeout", "10s", "-vet=off", pkg},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.inv.args(); !slices.Equal(got, tt.want) {
				t.Fatalf("args()\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestImportPathsResolvesEachPackageOnce(t *testing.T) {
	var lookups []string
	paths := &importPaths{lookup: func(dir string) string {
		lookups = append(lookups, dir)
		return "example.com/m/" + filepath.Base(dir)
	}}
	a := []string{"/src/m/a/a.go", "/src/m/a/b.go"}
	for range 3 {
		if got := paths.forFiles(a); got != "example.com/m/a" {
			t.Fatalf("forFiles(a) = %q, want example.com/m/a", got)
		}
	}
	if got := paths.forFiles([]string{"/src/m/c/c.go"}); got != "example.com/m/c" {
		t.Fatalf("forFiles(c) = %q, want example.com/m/c", got)
	}
	if want := []string{"/src/m/a", "/src/m/c"}; !slices.Equal(lookups, want) {
		t.Fatalf("lookups = %q, want %q", lookups, want)
	}
}

func TestImportPathsCachesFailedLookup(t *testing.T) {
	calls := 0
	paths := &importPaths{lookup: func(string) string { calls++; return "" }}
	paths.forFiles([]string{"/src/m/broken/x.go"})
	if got := paths.forFiles([]string{"/src/m/broken/x.go"}); got != "" || calls != 1 {
		t.Fatalf("forFiles = %q after %d lookups, want \"\" after 1", got, calls)
	}
}

func TestImportPathsNoFiles(t *testing.T) {
	paths := &importPaths{lookup: func(string) string { t.Fatal("unexpected lookup"); return "" }}
	if got := paths.forFiles(nil); got != "" {
		t.Fatalf("forFiles(nil) = %q, want empty", got)
	}
}
