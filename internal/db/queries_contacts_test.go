package db

import "testing"

func TestRankForOrder(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "Primary"},
		{1, "Secondary"},
		{2, "Tertiary"},
		// A 4th+ contact of the same type must not panic or return "" --
		// it clamps to the last defined rank rather than erroring.
		{3, "Tertiary"},
		{100, "Tertiary"},
	}
	for _, c := range cases {
		if got := rankForOrder(c.n); got != c.want {
			t.Errorf("rankForOrder(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
