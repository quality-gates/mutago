//go:build examplemain
// +build examplemain

package main

type Point struct {
	X, Y int
}

func makePoint() Point {
	return Point{X: 1, Y: 2}
}

func main()	{}
