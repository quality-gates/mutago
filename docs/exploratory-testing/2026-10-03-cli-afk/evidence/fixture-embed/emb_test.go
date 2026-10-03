package emb

import "testing"

func TestGreeting(t *testing.T) {
	if Greeting != "hello" {
		t.Fatalf("Greeting = %q", Greeting)
	}
}

func TestPick(t *testing.T) {
	if Pick(1, 1) != 1 {
		t.Fatal("pick")
	}
}
