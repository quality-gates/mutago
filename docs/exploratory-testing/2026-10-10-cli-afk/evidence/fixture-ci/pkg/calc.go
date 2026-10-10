package pkg

func Inc(x int) int {
	return x + 1
}

func IncAgain(x int) int {
	return x + 1
}

func Clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
