//go:build examplemain
// +build examplemain

package main

type Config struct {
	Timeout int
}
type MyConfig = Config

func Get(b bool) MyConfig {
	if b {
		return MyConfig{Timeout: 1}
	} else {
		return Config{}
	}
}
