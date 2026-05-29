//go:build examplemain
// +build examplemain

package main

import "fmt"

func main() {
	ch := make(chan int)

	select {
	case v := <-ch:
		fmt.Println(v)
	default:
		fmt.Println("default")
	}
}
