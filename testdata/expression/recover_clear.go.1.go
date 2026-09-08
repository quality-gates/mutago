//go:build examplemain
// +build examplemain

package main

import "fmt"

func guarded() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered:", r)
		}
	}()
	panic("boom")
}

func bare() {
	defer func() {
		_ = func() any { return nil }()
	}()
	panic("boom")
}

func direct() {
	defer recover()
}

func main() {
	guarded()
	bare()
	direct()
}
