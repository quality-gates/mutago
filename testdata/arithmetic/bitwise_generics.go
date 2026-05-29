//go:build examplemain
// +build examplemain

package main

// These types are used only in the type union constraint below.
type bwgA struct{ val int }
type bwgB struct{ val string }
type bwgC struct{ val float64 }

// bwgPayloads is a union type constraint. The | operators here are
// type-level (not bitwise) and must never be mutated by arithmetic/bitwise.
type bwgPayloads interface {
	*bwgA | *bwgB | *bwgC
}

func main() {}
