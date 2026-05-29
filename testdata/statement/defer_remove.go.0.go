//go:build examplemain
// +build examplemain

package main

func cleanup()	{}

func foo() {
	cleanup()
	defer cleanup()
}

func main()	{}
