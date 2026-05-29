//go:build examplemain
// +build examplemain

package main

import (
	"errors"
	"fmt"
)

func mayFail(fail bool) error {
	if fail {
		return errors.New("failure")
	}
	return nil
}

func main() {
	err := mayFail(true)
	if err != nil {
		fmt.Println("error:", err)
	}
	err2 := mayFail(false)
	if err2 == nil {
		fmt.Println("ok")
	}
}
