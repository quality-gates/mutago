//go:build examplemain
// +build examplemain

package main

import "time"

func Wait(ch chan int) {
	select {
	case <-ch:
	case <-time.After(time.Second):
	}
}

func main()	{}
