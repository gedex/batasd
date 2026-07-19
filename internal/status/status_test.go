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

func TestTerminal(t *testing.T) {
	if Terminal(Queued) {
		t.Fatal("Terminal(queued) = true, want false")
	}
	if Terminal(Processing) {
		t.Fatal("Terminal(processing) = true, want false")
	}
	if !Terminal(Accepted) {
		t.Fatal("Terminal(accepted) = false, want true")
	}
	if !Terminal("unknown_finished_state") {
		t.Fatal("Terminal(unknown) = false, want true")
	}
}
