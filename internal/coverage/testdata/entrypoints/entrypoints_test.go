package entrypoints

import "testing"

func TestValue(t *testing.T) {
	if Value() != 1 {
		t.Fatal("unexpected value")
	}
}

func FuzzValue(f *testing.F) {
	f.Add(1)
	f.Fuzz(func(t *testing.T, got int) {
		_ = got
	})
}

func BenchmarkValue(b *testing.B) {
	for b.Loop() {
		_ = Value()
	}
}
