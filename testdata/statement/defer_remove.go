//go:build examplemain
// +build examplemain

package main

func cleanup()	{}

func foo() {
	defer cleanup()
	defer cleanup()
}

func main()	{}
