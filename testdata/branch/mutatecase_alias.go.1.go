//go:build examplemain
// +build examplemain

package main

type Config struct {
	Timeout int
}
type MyConfig = Config

func Switch(n int) MyConfig {
	switch n {
	case 1:
		return MyConfig{Timeout: 1}
	default:
		return Config{}
	}
}
