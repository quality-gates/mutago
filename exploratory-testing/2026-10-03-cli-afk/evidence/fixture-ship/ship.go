package ship

func Fee(total int) int {
	if total >= 50 {
		return 0
	}
	return 5
}
