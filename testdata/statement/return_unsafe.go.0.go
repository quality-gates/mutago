//go:build examplemain
// +build examplemain

package main

import "unsafe"

func makeUnsafe() unsafe.Pointer {
	x := 42
	_ = x
	return nil
}

func main()	{}
