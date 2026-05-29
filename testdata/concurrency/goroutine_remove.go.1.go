//go:build examplemain
// +build examplemain

package main

import "fmt"

func worker() {
	fmt.Println("working")
}

func main() {
	go worker()
	worker()
}
