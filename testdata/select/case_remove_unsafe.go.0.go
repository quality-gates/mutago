//go:build examplemain
// +build examplemain

package main

import "time"

func Wait(ch chan int) {
	select {

	case <-time.After(time.Second):
	}
}

func main()	{}
