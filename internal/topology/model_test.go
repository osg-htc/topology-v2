package topology

import "testing"

func TestResourceActive(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name string
		in   *bool
		want bool
	}{
		{"omitted defaults to true", nil, true},
		{"explicit true", &yes, true},
		{"explicit false", &no, false},
	}
	for _, c := range cases {
		if got := ResourceActive(c.in); got != c.want {
			t.Errorf("%s: ResourceActive(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

func TestResourceDisabled(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name string
		in   *bool
		want bool
	}{
		{"omitted defaults to false", nil, false},
		{"explicit true", &yes, true},
		{"explicit false", &no, false},
	}
	for _, c := range cases {
		if got := ResourceDisabled(c.in); got != c.want {
			t.Errorf("%s: ResourceDisabled(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}
