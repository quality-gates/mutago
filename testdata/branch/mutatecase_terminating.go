//go:build examplemain
// +build examplemain

package main

func Switch(n int) string {
	switch n {
	case 1:
		return "one"
	default:
		return "other"
	}
}
