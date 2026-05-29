//go:build examplemain
// +build examplemain

package main

func f(a, b bool) bool {
	if !a {
		return false
	}
	if !a && b {
		return false
	}
	return true
}

func main()	{}
