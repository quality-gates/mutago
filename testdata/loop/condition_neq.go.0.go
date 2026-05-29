//go:build examplemain
// +build examplemain

package main

import "fmt"

func main() {
	k := 5

	for 1 < 1 {
		k = k - 1
	}

	fmt.Println(k)
}
