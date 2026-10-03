package ann

func Cap(pct int) int {
	// mutator-disable-next-line branch/if
	if pct > 100 {
		pct = 100
	}
	return pct
}

func Bump(x int) int {
	y := 0
	// mutator-disable-next-line branch/if, increment
	if x > 0 {
		y += 1
	}
	return y
}
