//go:build examplemain
// +build examplemain

package main

type Config struct {
	Timeout	int
	Retries	int
	Name	string
	Debug	bool
}

func build() Config {
	return Config{Timeout: 30, Retries: 3, Name: "x", Debug: false}
}

func counts() map[string]int {
	return map[string]int{"a": 1, "b": 0}
}

func main() {
	_ = build()
	_ = counts()
}
