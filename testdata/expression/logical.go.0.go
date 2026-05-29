//go:build examplemain
// +build examplemain

package main

import "fmt"

func main() {
	i := 1
	j := 2

	if i == 1 || j == 2 {
		fmt.Println("both")
	}
	if i == 1 || j == 2 {
		fmt.Println("either")
	}
	fmt.Println("done")
}
