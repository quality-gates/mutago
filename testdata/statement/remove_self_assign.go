//go:build examplemain
// +build examplemain

package main

import "fmt"

func main() {
	x := 1
	// keep this comment attached to the self-assignment
	x = x
	y := 2
	// keep this second comment attached too
	y = y
	fmt.Println(x + y)
}
