//go:build examplemain
// +build examplemain

package main

import "fmt"

func main() {
	x := 1
	x = x
	y := 2
	y = y
	fmt.Println(x + y)
}
