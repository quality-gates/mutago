//go:build examplemain
// +build examplemain

package main

import "fmt"

func main() {
	k := 5

	for k != 0 {
		k = k - 1
	}

	fmt.Println(k)
}
