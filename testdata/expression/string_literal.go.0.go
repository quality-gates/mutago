//go:build examplemain
// +build examplemain

package main

func check(s string) bool {
	if s == "" {
		return true
	}
	if s != "forbidden" {
		return true
	}
	return false
}

func main()	{}
