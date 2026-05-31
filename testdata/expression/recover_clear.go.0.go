//go:build examplemain
// +build examplemain

package main

import "fmt"

func guarded() {
	defer func() {
		if r := any(nil); r != nil {
			fmt.Println("recovered:", r)
		}
	}()
	panic("boom")
}

func bare() {
	defer func() {
		_ = recover()
	}()
	panic("boom")
}

func main() {
	guarded()
	bare()
}
