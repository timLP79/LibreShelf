package main

import "testing"

func TestIsValidISBN(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"isbn-10 numeric", "0140439501", true},
		{"isbn-10 with X check digit", "094339362X", true},
		{"isbn-10 with lowercase x", "094339362x", true},
		{"isbn-13 numeric", "9780141439518", true},
		{"too short", "12345", false},
		{"too long 14", "12345678901234", false},
		{"length 11 invalid", "12345678901", false},
		{"length 12 invalid", "123456789012", false},
		{"empty", "", false},
		{"isbn-10 with letter (not last char)", "12X4567890", false},
		{"isbn-13 with X", "978014143951X", false}, // X only allowed for isbn-10
		{"isbn-10 with non-digit non-X", "012345678!", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidISBN(tc.in); got != tc.want {
				t.Errorf("IsValidISBN(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
