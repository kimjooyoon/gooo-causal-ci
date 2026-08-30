package subject

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d", got)
	}
}

func TestSubtract(t *testing.T) {
	if got := Subtract(7, 3); got != 4 {
		t.Fatalf("Subtract(7, 3) = %d", got)
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize(-8); got != 8 {
		t.Fatalf("Normalize(-8) = %d", got)
	}
}

func TestIntegration(t *testing.T) {
	if got := Describe(-4); got != "value=4" {
		t.Fatalf("Describe(-4) = %q", got)
	}
}
