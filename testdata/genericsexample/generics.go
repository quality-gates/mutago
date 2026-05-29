package genericsexample

// A, B, and C are concrete payload types.
type A struct{ Val int }
type B struct{ Val string }
type C struct{ Val float64 }

// PayloadTypes is a union constraint. The | operators are type-level and
// must not be treated as bitwise OR by the arithmetic/bitwise mutator.
type PayloadTypes interface {
	*A | *B | *C
}

// Wrap returns its argument as an interface value.
func Wrap[T PayloadTypes](t T) interface{} {
	return t
}
