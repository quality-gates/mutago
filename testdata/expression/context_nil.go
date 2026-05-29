//go:build examplemain
// +build examplemain

package main

import "context"

func process(ctx context.Context, val int) {
	_, _ = ctx, val
}

func wrap(ctx context.Context) {
	process(ctx, 1)
}

func main() {
	ctx := context.Background()
	wrap(ctx)
}
