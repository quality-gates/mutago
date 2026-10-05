package annotation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMutators(t *testing.T) {
	tests := []struct {
		name        string
		mutatorList string
		expected    []string
	}{
		{
			name:        "Valid list of mutators",
			mutatorList: "MutatorA, MutatorB, MutatorC",
			expected:    []string{"MutatorA", "MutatorB", "MutatorC"},
		},
		{
			name:        "List with leading and trailing spaces",
			mutatorList: "  MutatorA,  MutatorB , MutatorC  ",
			expected:    []string{"MutatorA", "MutatorB", "MutatorC"},
		},
		{
			name:        "Empty string",
			mutatorList: "",
			expected:    []string{},
		},
		{
			name:        "String with only commas",
			mutatorList: ",,,",
			expected:    []string{},
		},
		{
			name:        "Single mutator",
			mutatorList: "MutatorA",
			expected:    []string{"MutatorA"},
		},
		{
			name:        "Multiple empty elements",
			mutatorList: "MutatorA,,,MutatorB,,MutatorC,,",
			expected:    []string{"MutatorA", "MutatorB", "MutatorC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMutators(tt.mutatorList)
			if !assert.Equal(t, result, tt.expected) {
				t.Errorf("Expected %v, but got %v", tt.expected, result)
			}
		})
	}
}

func TestParseRegexAnnotation(t *testing.T) {
	tests := []struct {
		name          string
		commentText   string
		expectedRegex *regexp.Regexp
		expectedInfo  mutatorInfo
	}{
		{
			name:        "Valid regex and mutators",
			commentText: "RegexName ^[a-z]+$ MutatorA, MutatorB",
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile("^[a-z]+$")
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"MutatorA", "MutatorB"}),
		},
		{
			name:        "Valid regex without mutators",
			commentText: "RegexName ^[0-9]{4}$",
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile("^[0-9]{4}$")
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"*"}),
		},
		{
			name:        "Regex with spaces and wildcard mutator",
			commentText: "RegexName return true *",
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile("return true")
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"*"}),
		},
		{
			name:        "Regex with spaces and explicit mutators",
			commentText: "RegexName return true branch/if, conditional/negation",
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile("return true")
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"branch/if", "conditional/negation"}),
		},
		{
			name:        "Regex with spaces and single explicit mutator",
			commentText: "RegexName return true branch/if",
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile("return true")
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"branch/if"}),
		},
		{
			name:        "No-space regex with wildcard mutator",
			commentText: "RegexName return *",
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile("return")
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"*"}),
		},
		{
			name:        "Regex with escaped parens and wildcard mutator",
			commentText: `RegexName s\.Method\(\) *`,
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile(`s\.Method\(\)`)
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"*"}),
		},
		{
			name:        "Quoted regex with spaces and wildcard mutator",
			commentText: `RegexName "return true" *`,
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile("return true")
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"*"}),
		},
		{
			name:        "Regex with commas and explicit mutators",
			commentText: "RegexName foo, bar MutatorA, MutatorB",
			expectedRegex: func() *regexp.Regexp {
				re, _ := regexp.Compile("foo, bar")
				return re
			}(),
			expectedInfo: newMutatorInfo([]string{"MutatorA", "MutatorB"}),
		},
		{
			name:          "Invalid regex with wildcard",
			commentText:   "RegexName [a-z *",
			expectedRegex: nil,
			expectedInfo:  mutatorInfo{},
		},
		{
			name:          "Invalid regex with explicit mutator",
			commentText:   "RegexName [a-z branch/if",
			expectedRegex: nil,
			expectedInfo:  mutatorInfo{},
		},
		{
			name:          "Invalid regex",
			commentText:   "RegexName [a-z",
			expectedRegex: nil,
			expectedInfo:  mutatorInfo{},
		},
		{
			name:          "Empty comment",
			commentText:   "RegexName ",
			expectedRegex: nil,
			expectedInfo:  mutatorInfo{},
		},
	}

	r := &RegexAnnotation{Name: "RegexName"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultRegex, resultInfo := r.parseRegexAnnotation(tt.commentText)

			if resultRegex == nil && tt.expectedRegex != nil || resultRegex != nil && resultRegex.String() != tt.expectedRegex.String() {
				t.Errorf("Expected regex %v, but got %v", tt.expectedRegex, resultRegex)
			}

			assert.Equal(t, tt.expectedInfo, resultInfo)
		})
	}
}

func TestParseLineAnnotation(t *testing.T) {
	tests := []struct {
		name         string
		commentText  string
		expectedInfo mutatorInfo
	}{
		{
			name:         "Valid mutators",
			commentText:  "LineName MutatorA, MutatorB, MutatorC",
			expectedInfo: newMutatorInfo([]string{"MutatorA", "MutatorB", "MutatorC"}),
		},
		{
			name:         "Single mutator",
			commentText:  "LineName MutatorA",
			expectedInfo: newMutatorInfo([]string{"MutatorA"}),
		},
		{
			name:         "Multiple mutators with spaces",
			commentText:  "LineName  MutatorA, MutatorB , MutatorC  ",
			expectedInfo: newMutatorInfo([]string{"MutatorA", "MutatorB", "MutatorC"}),
		},
		{
			name:         "Empty comment",
			commentText:  "LineName ",
			expectedInfo: newMutatorInfo([]string{"*"}),
		},
		{
			name:         "Only spaces in mutators",
			commentText:  "LineName ,,,",
			expectedInfo: newMutatorInfo([]string{"*"}),
		},
		{
			name:         "Empty mutators",
			commentText:  "LineName",
			expectedInfo: newMutatorInfo([]string{"*"}),
		},
	}

	l := &LineAnnotation{Name: "LineName"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := l.parseLineAnnotation(tt.commentText)

			assert.Equal(t, tt.expectedInfo, result)
		})
	}
}

func TestExistsFuncAnnotation(t *testing.T) {
	tests := []struct {
		name          string
		funcDecl      *ast.FuncDecl
		expectedExist bool
	}{
		{
			name: "No annotations",
			funcDecl: &ast.FuncDecl{
				Doc: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: "// Some other comment"},
					},
				},
			},
			expectedExist: false,
		},
		{
			name: "Valid annotation exists",
			funcDecl: &ast.FuncDecl{
				Doc: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: "// mutator-disable-func something here"},
					},
				},
			},
			expectedExist: true,
		},
		{
			name: "Multiple comments, valid annotation exists",
			funcDecl: &ast.FuncDecl{
				Doc: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: "// Another comment"},
						{Text: "// mutator-disable-func something here"},
					},
				},
			},
			expectedExist: true,
		},
		{
			name: "Doc is nil",
			funcDecl: &ast.FuncDecl{
				Doc: nil,
			},
			expectedExist: false,
		},
	}

	p := NewProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.existsFuncAnnotation(tt.funcDecl)
			if result != tt.expectedExist {
				t.Errorf("Expected %v, but got %v", tt.expectedExist, result)
			}
		})
	}
}

func TestCollectFunctionsAndFilterFunctions(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		expected  bool
		filterPos token.Pos
	}{
		{
			name:      "Function with one statement",
			code:      `package main; func test() { var a = 10 }`,
			expected:  true,
			filterPos: token.Pos(1),
		},
		{
			name:      "Function with nested statements",
			code:      `package main; func test() { if true { var a = 10 } }`,
			expected:  true,
			filterPos: token.Pos(2),
		},
	}

	f := &FunctionAnnotation{Exclusions: make(map[token.Pos]struct{})}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := token.NewFileSet()
			node, err := parser.ParseFile(fs, "func_annotation_test.go", tt.code, parser.Mode(0))
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			// Находим первую функцию в коде
			var funcDecl *ast.FuncDecl
			ast.Inspect(node, func(n ast.Node) bool {
				if fDecl, ok := n.(*ast.FuncDecl); ok {
					funcDecl = fDecl
					return false
				}
				return true
			})

			f.collectFunctions(funcDecl)

			filtered := f.filterFunctions(funcDecl)
			assert.Equal(t, tt.expected, filtered)

		})
	}
}

func TestFindLinesMatchedRegex(t *testing.T) {
	tests := []struct {
		name          string
		filePath      string
		re            *regexp.Regexp
		expectedLines []int
	}{
		{
			name:          "No regex match",
			filePath:      "../../testdata/annotation/regex.go",
			re:            regexp.MustCompile("notmatching"),
			expectedLines: []int{},
		},
		{
			name:          "Match variable declaration",
			filePath:      "../../testdata/annotation/regex.go",
			re:            regexp.MustCompile(`test`),
			expectedLines: []int{37, 38},
		},
		{
			name:          "Match Println",
			filePath:      "../../testdata/annotation/regex.go",
			re:            regexp.MustCompile(`Println`),
			expectedLines: []int{16, 17, 23, 33, 38},
		},
		{
			name:          "Multiple matches on multiple lines",
			filePath:      "../../testdata/annotation/regex.go",
			re:            regexp.MustCompile(`xx+`),
			expectedLines: []int{12, 13, 16, 17},
		},
		{
			name:          "Match MyStruct",
			filePath:      "../../testdata/annotation/regex.go",
			re:            regexp.MustCompile(`MyStruct`),
			expectedLines: []int{19, 27, 31},
		},
		{
			name:          "Match interface declaration",
			filePath:      "../../testdata/annotation/regex.go",
			re:            regexp.MustCompile(`interface`),
			expectedLines: []int{42},
		},
		{
			name:          "Match slog.Info",
			filePath:      "../../testdata/annotation/regex.go",
			re:            regexp.MustCompile(`slog\.Info`),
			expectedLines: []int{49},
		},
		{
			name:          "Match s.Method()",
			filePath:      "../../testdata/annotation/regex.go",
			re:            regexp.MustCompile(`s\.Method\(\)`),
			expectedLines: []int{20},
		},
		{
			name:          "Regex is nil",
			filePath:      "../../testdata/annotation/regex.go",
			re:            nil,
			expectedLines: []int{},
		},
		{
			name:          "Empty file",
			filePath:      "../../testdata/annotation/empty.go",
			re:            regexp.MustCompile(`test`),
			expectedLines: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			r := &RegexAnnotation{Name: "TestRegexAnnotation"}

			actual, _ := r.findLinesMatchingRegex(tt.filePath, tt.re)

			assert.ElementsMatch(t, tt.expectedLines, actual)
		})
	}
}

func TestCollect(t *testing.T) {
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "../../testdata/annotation/collect.go", nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	processor := NewProcessor()

	processor.Collect(file, fs, "../../testdata/annotation/collect.go")

	assert.NotEmpty(t, processor.FunctionAnnotation.Exclusions)
	assert.Equal(t, processor.FunctionAnnotation.Exclusions, map[token.Pos]struct{}{
		75: {}, 99: {}, 104: {}, 114: {}, 115: {}, 117: {}, 122: {}, 126: {}, 129: {}, 136: {}, 140: {},
	})

	assert.NotEmpty(t, processor.RegexAnnotation.Exclusions)
	assert.Equal(t, processor.RegexAnnotation.Exclusions, map[int]map[token.Pos]mutatorInfo{
		14: {
			169: newMutatorInfo([]string{"*"}),
			173: newMutatorInfo([]string{"*"}),
			181: newMutatorInfo([]string{"*"}),
		},
		22: {
			304: newMutatorInfo([]string{"*"}),
			308: newMutatorInfo([]string{"*"}),
			316: newMutatorInfo([]string{"*"}),
		},
		21: {
			288: newMutatorInfo([]string{"*"}),
			292: newMutatorInfo([]string{"*"}),
			300: newMutatorInfo([]string{"*"}),
		},
	})

	assert.NotEmpty(t, processor.LineAnnotation.Exclusions)
	assert.Equal(t, processor.LineAnnotation.Exclusions, map[int]map[token.Pos]mutatorInfo{
		19: {
			275: newMutatorInfo([]string{"numbers/incrementer"}),
			279: newMutatorInfo([]string{"numbers/incrementer"}),
			283: newMutatorInfo([]string{"numbers/incrementer"}),
		},
	})

}

func TestCollectRegexAnnotationUsesPhysicalLinesWithLineDirective(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directive string
	}{
		{name: "named", directive: "//line fake.go:100"},
		{name: "filename-less", directive: "//line :100"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := "package p\n" + tt.directive + "\n// mutator-disable-regexp \"return 1\" *\nfunc f() int { return 1 }\n"
			tmp := filepath.Join(t.TempDir(), "sample.go")
			if err := os.WriteFile(tmp, []byte(src), 0644); err != nil {
				t.Fatal(err)
			}

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tmp, src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}

			processor := NewProcessor()
			processor.Collect(file, fset, tmp)

			statement := file.Decls[0].(*ast.FuncDecl).Body.List[0]
			assert.True(t, processor.ShouldSkip(statement, "statement/remove"), "regex annotation should target the physical return line")
		})
	}
}

func TestCollectNextLineAnnotationUsesPhysicalLinesWithLineDirective(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directive string
	}{
		{name: "named", directive: "//line fake.go:100"},
		{name: "filename-less", directive: "//line :100"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := "package p\n" + tt.directive + "\n// mutator-disable-next-line *\nfunc f() { return }\n"
			tmp := filepath.Join(t.TempDir(), "sample.go")
			if err := os.WriteFile(tmp, []byte(src), 0644); err != nil {
				t.Fatal(err)
			}

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tmp, src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}

			processor := NewProcessor()
			processor.Collect(file, fset, tmp)

			statement := file.Decls[0].(*ast.FuncDecl).Body.List[0]
			assert.True(t, processor.ShouldSkip(statement, "statement/remove"), "next-line annotation should target the following physical source line")
		})
	}
}

func collectAnnotationFixture(t *testing.T, src string) (*Processor, *ast.File) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(tmp, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, tmp, src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	processor := NewProcessor()
	processor.Collect(file, fset, tmp)
	return processor, file
}

func TestOmittedMutatorListDisablesEveryMutatorOnTargetLines(t *testing.T) {
	const src = "package pkg\n\nfunc Add(a, b int) int {\n\t// mutator-disable-next-line\n\tc := a + 1\n\treturn c + b\n}\n"
	processor, file := collectAnnotationFixture(t, src)
	fn := file.Decls[0].(*ast.FuncDecl)
	assign := fn.Body.List[0].(*ast.AssignStmt)
	add := assign.Rhs[0].(*ast.BinaryExpr)
	one := add.Y
	ret := fn.Body.List[1].(*ast.ReturnStmt)
	laterAdd := ret.Results[0].(*ast.BinaryExpr)

	for _, mutatorName := range []string{"arithmetic/base", "numbers/incrementer", "numbers/decrementer", "statement/remove"} {
		assert.True(t, processor.ShouldSkip(assign, mutatorName), "bare next-line should suppress %s on the following line", mutatorName)
		assert.True(t, processor.ShouldSkip(add, mutatorName), "bare next-line should suppress %s on the following line's expression", mutatorName)
		assert.True(t, processor.ShouldSkip(one, mutatorName), "bare next-line should suppress %s on the following line's literal", mutatorName)
		assert.True(t, HandleBlockStmt(assign, mutatorName), "bare next-line should suppress %s via the statement filter", mutatorName)
		assert.False(t, processor.ShouldSkip(laterAdd, mutatorName), "bare next-line must not suppress %s on a later line", mutatorName)
		assert.False(t, HandleBlockStmt(ret, mutatorName), "bare next-line must not suppress %s on a later statement", mutatorName)
	}
}

func TestPatternOnlyRegexDisablesEveryMutatorOnMatchingLines(t *testing.T) {
	const src = "package pkg\n\n// mutator-disable-regexp Mutated\nfunc Add(a, b int) int {\n\tc := a + 1 // Mutated\n\treturn c + b\n}\n"
	processor, file := collectAnnotationFixture(t, src)
	fn := file.Decls[0].(*ast.FuncDecl)
	assign := fn.Body.List[0].(*ast.AssignStmt)
	add := assign.Rhs[0].(*ast.BinaryExpr)
	ret := fn.Body.List[1].(*ast.ReturnStmt)
	laterAdd := ret.Results[0].(*ast.BinaryExpr)

	for _, mutatorName := range []string{"arithmetic/base", "numbers/incrementer", "statement/return"} {
		assert.True(t, processor.ShouldSkip(add, mutatorName), "pattern-only regex should suppress %s on a matching line", mutatorName)
		assert.True(t, HandleBlockStmt(assign, mutatorName), "pattern-only regex should suppress %s via the statement filter", mutatorName)
		assert.False(t, processor.ShouldSkip(laterAdd, mutatorName), "pattern-only regex must not suppress %s on a non-matching line", mutatorName)
	}
}

func TestExplicitMutatorListStaysSelectiveWhenListIsPresent(t *testing.T) {
	const src = "package pkg\n\nfunc Add(a, b int) int {\n\t// mutator-disable-next-line arithmetic/base\n\tc := a + 1\n\treturn c + b\n}\n"
	processor, file := collectAnnotationFixture(t, src)
	fn := file.Decls[0].(*ast.FuncDecl)
	add := fn.Body.List[0].(*ast.AssignStmt).Rhs[0].(*ast.BinaryExpr)

	assert.True(t, processor.ShouldSkip(add, "arithmetic/base"))
	assert.False(t, processor.ShouldSkip(add, "numbers/incrementer"))
}

func TestNextLinePrefixWildcardSuppressesEveryMutatorWithThatPrefix(t *testing.T) {
	const src = "package pkg\n\nfunc Add(a, b int) int {\n\t// mutator-disable-next-line numbers/*\n\tc := a + 1\n\treturn c + b\n}\n"
	processor, file := collectAnnotationFixture(t, src)
	fn := file.Decls[0].(*ast.FuncDecl)
	one := fn.Body.List[0].(*ast.AssignStmt).Rhs[0].(*ast.BinaryExpr).Y

	assert.True(t, processor.ShouldSkip(one, "numbers/incrementer"))
	assert.True(t, processor.ShouldSkip(one, "numbers/decrementer"))
	assert.False(t, processor.ShouldSkip(one, "arithmetic/base"))
}

func TestRegexPrefixWildcardSuppressesEveryMutatorWithThatPrefix(t *testing.T) {
	const src = "package pkg\n\n// mutator-disable-regexp Mutated numbers/*\nfunc Add(a, b int) int {\n\tc := a + 1 // Mutated\n\treturn c + b\n}\n"
	processor, file := collectAnnotationFixture(t, src)
	fn := file.Decls[0].(*ast.FuncDecl)
	one := fn.Body.List[0].(*ast.AssignStmt).Rhs[0].(*ast.BinaryExpr).Y

	assert.True(t, processor.ShouldSkip(one, "numbers/incrementer"))
	assert.True(t, processor.ShouldSkip(one, "numbers/decrementer"))
	assert.False(t, processor.ShouldSkip(one, "arithmetic/base"))
}
