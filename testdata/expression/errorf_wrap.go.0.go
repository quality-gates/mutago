//go:build examplemain
// +build examplemain

package main

import (
	"errors"
	"fmt"
)

func wrapped() error {
	err := errors.New("boom")
	return fmt.Errorf("load config: %v", err)
}

func notWrapped() error {
	err := errors.New("boom")
	return fmt.Errorf("load config: %v", err)
}

func doubleWrapped() error {
	e1 := errors.New("a")
	e2 := errors.New("b")
	return fmt.Errorf("both failed: %w and %w", e1, e2)
}

func main() {
	_ = wrapped()
	_ = notWrapped()
	_ = doubleWrapped()
}
