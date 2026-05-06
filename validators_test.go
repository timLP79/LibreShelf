// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"strings"
	"testing"
)

// TestValidateUsername pins the rules from DEC-021 (and the username half
// of the policy in validators.go): 3-32 chars, letters/digits/underscore
// only. Table-driven so each rule edge has a named, failing-on-regression
// case. If a future change relaxes one of these rules silently, the
// matching row here fires.
func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		errMatch string // substring the error should contain; ignored when wantErr is false
	}{
		{"empty", "", true, "3 characters"},
		{"too short (2 chars)", "ab", true, "3 characters"},
		{"minimum length (3 chars)", "abc", false, ""},
		{"valid alphanumeric", "staff1", false, ""},
		{"valid with underscore", "admin_user", false, ""},
		{"maximum length (32 chars)", strings.Repeat("a", 32), false, ""},
		{"too long (33 chars)", strings.Repeat("a", 33), true, "32 characters"},
		{"rejects space", "bad name", true, "letters, numbers"},
		{"rejects hyphen", "bad-name", true, "letters, numbers"},
		{"rejects dot", "bad.name", true, "letters, numbers"},
		{"rejects special char", "bad!name", true, "letters, numbers"},
		{"rejects non-ASCII letter", "bad\u00e9name", true, "letters, numbers"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUsername(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
				return
			}
			if tc.wantErr && tc.errMatch != "" && !strings.Contains(err.Error(), tc.errMatch) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.errMatch)
			}
		})
	}
}

// TestValidatePassword pins the DEC-021 password complexity rule: 8+
// characters with at least one uppercase letter, one digit, and one
// character that is unicode.IsPunct or unicode.IsSymbol. The switch in
// ValidatePassword makes each rune fall into exactly one category, so
// lowercase letters do not satisfy any of the three required categories.
func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		errMatch string
	}{
		{"empty", "", true, "8 characters"},
		{"too short (7 chars)", "Ab1!abc", true, "8 characters"},
		{"minimum valid (8 chars)", "Ab1!abcd", false, ""},
		{"no uppercase", "ab1!abcd", true, "uppercase"},
		{"no digit", "Abcd!abcd", true, "digit"},
		{"no special", "Abcd1abcd", true, "special"},
		{"all requirements met", "Admin123!", false, ""},
		{"special via IsSymbol (+)", "Admin123+", false, ""},
		{"special via IsPunct (@)", "Admin123@", false, ""},
		{"longer valid", "CorrectHorseBatteryStaple1!", false, ""},
		{"uppercase-only fails missing digit", "PASSWORD!", true, "digit"},
		{"digit-only fails missing uppercase", "password1!", true, "uppercase"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
				return
			}
			if tc.wantErr && tc.errMatch != "" && !strings.Contains(err.Error(), tc.errMatch) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.errMatch)
			}
		})
	}
}

// TestGenerateBaseUsername pins the auto-generated-username rules for
// patron create: first-initial + last-word for multi-word names, whole
// word for single-word names, lowercased, with non-alphanumerics
// stripped so the output always satisfies the ValidateUsername regex
// [a-zA-Z0-9_]+ (the _ can never appear here but the output is still
// regex-compliant by construction). Each case pins a specific branch
// so a future edit that breaks one row fires clearly.
func TestGenerateBaseUsername(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"two-word common case", "Jane Smith", "jsmith"},
		{"three-word picks first+last", "Jane Mary Smith", "jsmith"},
		{"single-word keeps whole name", "Cher", "cher"},
		{"hyphenated last name strips hyphen", "Ahmed Al-Farsi", "aalfarsi"},
		{"apostrophe in last name stripped", "Mary O'Brien", "mobrien"},
		{"leading and trailing whitespace ignored", "  Jane   Smith  ", "jsmith"},
		{"mixed case normalized to lower", "JANE SMITH", "jsmith"},
		{"digits in name retained", "R2D2 Droid", "rdroid"},
		{"accented letter stripped", "Jean Café", "jcaf"},
		{"empty input returns empty", "", ""},
		{"whitespace-only returns empty", "   ", ""},
		{"single word with punctuation", "O'Brien", "obrien"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := generateBaseUsername(tc.input); got != tc.want {
				t.Errorf("generateBaseUsername(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
