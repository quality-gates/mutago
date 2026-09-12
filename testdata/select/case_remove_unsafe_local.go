//go:build examplemain
// +build examplemain

package main

func Send(ch chan int, x int) {
	v := x + 1
	select {
	case <-ch:
	case ch <- v:
	}
}

func main()	{}
