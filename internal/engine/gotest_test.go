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
			name: "mutant reuses one executable via -exec",
			inv:  goTestInvocation{kind: mutantRun, target: pkg, timeoutSeconds: 5, overlay: "o.json", execProgram: "/tmp/mutago-stable-exec.sh"},
			want: []string{"test", "-overlay=o.json", "-timeout", "5s", "-vet=off", "-exec=/tmp/mutago-stable-exec.sh", "-failfast", pkg},
		},
		{
			name: "user -exec wins over the stable executable wrapper",
			inv:  goTestInvocation{kind: mutantRun, target: pkg, timeoutSeconds: 5, overlay: "o.json", testFlags: []string{"-exec=/usr/bin/true"}, execProgram: "/tmp/mutago-stable-exec.sh"},
			want: []string{"test", "-overlay=o.json", "-timeout", "5s", "-exec=/usr/bin/true", "-vet=off", "-failfast", pkg},
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
		{
			name: "list tests keeps only build flags",
			inv:  goTestInvocation{kind: listTestsRun, target: pkg, timeoutSeconds: 30, testFlags: []string{"-tags", "integration", "--count", "2", "-race", "-run", "TestX", "-vet=all"}},
			want: []string{"test", "-list", ".*", "-timeout", "30s", "-tags", "integration", "-race", "-vet=off", pkg},
		},
		{
			name: "compile test binary keeps every user flag",
			inv:  goTestInvocation{kind: compileTestBinary, target: pkg + "/sub", timeoutSeconds: 30, coverPkg: pkg, binaryPath: "/tmp/b/tests", testFlags: []string{"-tags=integration", "--count=2"}},
			want: []string{"test", "-c", "-cover", "-covermode=set", "-coverpkg=" + pkg, "-o", "/tmp/b/tests", "-timeout", "30s", "-tags=integration", "--count=2", "-vet=off", pkg + "/sub"},
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

func TestPerTestToolchainCommands(t *testing.T) {
	const pkg = "example.com/m/pkg"
	tests := []struct {
		name      string
		testFlags []string
		list      []string
		packages  []string
		compile   []string
		run       []string
	}{
		{
			name:     "no user flags",
			list:     []string{"go", "test", "-list", ".*", "-timeout", "30s", "-vet=off", pkg},
			packages: []string{"go", "list", pkg + "/..."},
			compile:  []string{"go", "test", "-c", "-cover", "-covermode=set", "-coverpkg=" + pkg, "-o", "/b/tests", "-timeout", "30s", "-vet=off", pkg},
			run:      []string{"/b/tests", "-test.run=^TestA$", "-test.coverprofile=/b/TestA/c.out", "-test.timeout=30s"},
		},
		{
			name:      "tags",
			testFlags: []string{"-tags=integration"},
			list:      []string{"go", "test", "-list", ".*", "-timeout", "30s", "-tags=integration", "-vet=off", pkg},
			packages:  []string{"go", "list", "-tags=integration", pkg + "/..."},
			compile:   []string{"go", "test", "-c", "-cover", "-covermode=set", "-coverpkg=" + pkg, "-o", "/b/tests", "-timeout", "30s", "-tags=integration", "-vet=off", pkg},
			run:       []string{"/b/tests", "-test.run=^TestA$", "-test.coverprofile=/b/TestA/c.out", "-test.timeout=30s"},
		},
		{
			name:      "count with double dash",
			testFlags: []string{"--count", "2"},
			list:      []string{"go", "test", "-list", ".*", "-timeout", "30s", "-vet=off", pkg},
			packages:  []string{"go", "list", pkg + "/..."},
			compile:   []string{"go", "test", "-c", "-cover", "-covermode=set", "-coverpkg=" + pkg, "-o", "/b/tests", "-timeout", "30s", "--count", "2", "-vet=off", pkg},
			run:       []string{"/b/tests", "-test.count=2", "-test.run=^TestA$", "-test.coverprofile=/b/TestA/c.out", "-test.timeout=30s"},
		},
		{
			name:      "race",
			testFlags: []string{"-race"},
			list:      []string{"go", "test", "-list", ".*", "-timeout", "30s", "-race", "-vet=off", pkg},
			packages:  []string{"go", "list", "-race", pkg + "/..."},
			compile:   []string{"go", "test", "-c", "-cover", "-covermode=set", "-coverpkg=" + pkg, "-o", "/b/tests", "-timeout", "30s", "-race", "-vet=off", pkg},
			run:       []string{"/b/tests", "-test.run=^TestA$", "-test.coverprofile=/b/TestA/c.out", "-test.timeout=30s"},
		},
		{
			name:      "run",
			testFlags: []string{"-run", "TestB"},
			list:      []string{"go", "test", "-list", ".*", "-timeout", "30s", "-vet=off", pkg},
			packages:  []string{"go", "list", pkg + "/..."},
			compile:   []string{"go", "test", "-c", "-cover", "-covermode=set", "-coverpkg=" + pkg, "-o", "/b/tests", "-timeout", "30s", "-run", "TestB", "-vet=off", pkg},
			run:       []string{"/b/tests", "-test.run=TestB", "-test.run=^TestA$", "-test.coverprofile=/b/TestA/c.out", "-test.timeout=30s"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := perTestToolchain{testFlags: tt.testFlags, timeoutSeconds: 30}
			for _, c := range []struct {
				kind string
				got  []string
				want []string
			}{
				{"list tests", tc.ListTests(pkg).Args, tt.list},
				{"list packages", tc.ListPackages(pkg + "/...").Args, tt.packages},
				{"compile", tc.CompileTestBinary(pkg, pkg, "/b/tests").Args, tt.compile},
				{"run", tc.RunTest("/b/tests", "TestA", "/b/TestA/c.out").Args, tt.run},
			} {
				if !slices.Equal(c.got, c.want) {
					t.Errorf("%s\n got %q\nwant %q", c.kind, c.got, c.want)
				}
			}
		})
	}
}
