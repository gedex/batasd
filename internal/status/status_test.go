package status

import "testing"

func TestValid(t *testing.T) {
	if !Valid(Accepted) {
		t.Fatalf("Valid(%q) = false, want true", Accepted)
	}
	if Valid("not_real") {
		t.Fatal("Valid(not_real) = true, want false")
	}
}
