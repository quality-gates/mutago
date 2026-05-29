//go:build examplemain
// +build examplemain

package main

import "fmt"

func main() {
	ch := make(chan int, 1)
	done := make(chan struct{})
	ch <- 1

	select {
	case v := <-ch:
		fmt.Println(v)

	}
}
