//go:build examplemain
// +build examplemain

package main

func makeIntPtr() *int {
	x := 42
	_ = x
	return nil
}

func makeSlice() []int {
	return []int{1, 2, 3}
}

func main()	{}
