//go:build examplemain
// +build examplemain

package main

type Empty struct{ X int }

func alreadyZero() Empty {
	return Empty{}
}

func main()	{}
