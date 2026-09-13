//go:build examplemain
// +build examplemain

package main

type Config struct {
	Timeout int
}
type MyConfig = Config

func Get(b bool) MyConfig {
	if b {
		return Config{}
	} else {
		return MyConfig{Timeout: 2}
	}
}

func main()	{}
