package annotation

import "go/token"

// FunctionAnnotation represents a collection of exclusions of function declarations.
type FunctionAnnotation struct {
	Exclusions map[token.Pos]struct{}
	Name       string
}

// LineAnnotation represents a collection of exclusions based on lines in the file.
type LineAnnotation struct {
	Exclusions map[int]map[token.Pos]mutatorInfo
	Name       string
}

// RegexAnnotation represents a collection of exclusions based on regex pattern matches.
type RegexAnnotation struct {
	Exclusions map[int]map[token.Pos]mutatorInfo
	Name       string
}
