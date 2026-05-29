//go:build examplemain
// +build examplemain

package main

func makeIntPtr() *int {
	x := 42
	return &x
}

func makeSlice() []int {
	return nil
}

func main()	{}
