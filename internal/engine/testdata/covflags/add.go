package covflags

// Add is a trivial function so arithmetic mutants have a clear kill path.
func Add(a, b int) int { return a + b }
