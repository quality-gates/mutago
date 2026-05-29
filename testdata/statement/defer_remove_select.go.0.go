//go:build examplemain
// +build examplemain

package main

func cleanup()	{}

func process(ch chan struct{}) {
	select {
	case <-ch:
		cleanup()
	}
}

func main()	{}
