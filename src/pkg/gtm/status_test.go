package gtm

import "testing"

func TestNodeStatusFromString(t *testing.T) {
	cases := map[string]NodeStatus{
		"":      NodeStatusUp,
		"up":    NodeStatusUp,
		"down":  NodeStatusDown,
		"drain": NodeStatusDrain,
	}

	for input, expected := range cases {
		got, err := NodeStatusFromString(input)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", input, err)
		}
		if got != expected {
			t.Fatalf("for %s expected %s got %s", input, expected, got)
		}
	}

	if _, err := NodeStatusFromString("invalid"); err == nil {
		t.Fatalf("expected error for invalid status")
	}
}
