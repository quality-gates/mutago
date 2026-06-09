package lineposition

func NearTop(x int) int {
	x = x
	return x
}

func BelowComment(y int) int {
	// This comment must not become the reported mutation line.
	y = y
	return y
}
