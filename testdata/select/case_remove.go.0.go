//go:build examplemain
// +build examplemain

package main

import "fmt"

func main() {
	ch := make(chan int, 1)
	done := make(chan struct{})
	ch <- 1
	close(done)

	select {

	case <-done:
		fmt.Println("done")
	}
}
