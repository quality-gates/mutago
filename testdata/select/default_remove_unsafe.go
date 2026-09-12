//go:build examplemain
// +build examplemain

package main

import (
	"fmt"
	"time"
)

func Wait(ch chan int) {
	select {
	case v := <-ch:
		fmt.Println(v)
	default:
		fmt.Println(time.Now())
	}
}

func main()	{}
