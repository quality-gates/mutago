//go:build examplemain
// +build examplemain

package main

func cleanup()	{}

func foo() {
	defer cleanup()
	cleanup()
}

func main()	{}
