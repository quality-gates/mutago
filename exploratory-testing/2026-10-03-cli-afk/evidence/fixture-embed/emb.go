package emb

import _ "embed"

// Greeting is embedded at build time.
//
//go:embed greeting.txt
var Greeting string

// Pick returns the larger of two ints.
func Pick(a, b int) int {
	if a > b {
		return a
	}
	return b
}
