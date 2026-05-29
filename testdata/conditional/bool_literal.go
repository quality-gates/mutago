//go:build examplemain
// +build examplemain

package main

func configure(debug bool)	{}

func main() {
	enabled := true
	_ = enabled
	configure(false)
}
