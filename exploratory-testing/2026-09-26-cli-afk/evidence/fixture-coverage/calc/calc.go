package calc

func Add(a, b int) int {
	return a + b
}

func IsPositive(n int) bool {
	if n > 0 {
		return true
	}
	return false
}

func Multiply(a, b int) int {
	return a * b
}

func Uncovered(n int) int {
	return n * 2
}
