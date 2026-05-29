//go:build examplemain
// +build examplemain

package main

func configure(debug bool)	{}

func main() {
	enabled := false
	_ = enabled
	configure(false)
}
