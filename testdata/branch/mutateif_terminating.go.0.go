//go:build examplemain
// +build examplemain

package main

func IfElse(c bool) int {
	if c {
		return 0
	} else {
		return 2
	}
}
