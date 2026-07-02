package agent

import "testing"

func TestAdapterSmallHelpers(t *testing.T) {
	if min(2, 5) != 2 || min(5, 2) != 2 || min(3, 3) != 3 {
		t.Fatalf("min returned unexpected result")
	}
	cases := map[string]string{
		"":             "<empty>",
		"short":        "****",
		"12345678":     "****",
		"1234567890ab": "1234****90ab",
	}
	for input, want := range cases {
		if got := maskToken(input); got != want {
			t.Fatalf("maskToken(%q) = %q, want %q", input, got, want)
		}
	}
}
