package db

import "testing"

func TestSanitizeUsername(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Jane.Doe@example.org", "jane.doe"},
		{"  Jane Smith  ", "jane-smith"},
		{"UPPER_case-Name123", "upper_case-name123"},
		{"weird!@#$%^&*()chars", "weird"},
		{"@leading-at-with-nothing-before", "leading-at-with-nothing-before"},
		{"...leading-and-trailing-separators---", "leading-and-trailing-separators"},
		{"", ""},
		{"   ", ""},
		{"!@#$%^&*()", ""},
	}
	for _, c := range cases {
		if got := SanitizeUsername(c.in); got != c.want {
			t.Errorf("SanitizeUsername(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNullString(t *testing.T) {
	if got := nullString(""); got != nil {
		t.Errorf("nullString(\"\") = %v, want nil", got)
	}
	if got := nullString("x"); got != "x" {
		t.Errorf("nullString(\"x\") = %v, want \"x\"", got)
	}
}

func TestNullBytes(t *testing.T) {
	if got := nullBytes(nil); got != nil {
		t.Errorf("nullBytes(nil) = %v, want nil", got)
	}
	if got := nullBytes([]byte{}); got != nil {
		t.Errorf("nullBytes(empty) = %v, want nil", got)
	}
	if got := nullBytes([]byte("x")); got == nil {
		t.Error("nullBytes(non-empty) = nil, want the bytes back")
	}
}

func TestDeref(t *testing.T) {
	if got := deref(nil); got != "" {
		t.Errorf("deref(nil) = %q, want \"\"", got)
	}
	s := "value"
	if got := deref(&s); got != "value" {
		t.Errorf("deref(&value) = %q, want %q", got, "value")
	}
}
