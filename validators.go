// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"unicode"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

func ValidateUsername(s string) error {
	if len(s) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if len(s) > 32 {
		return fmt.Errorf("username must be 32 characters or fewer")
	}
	if !usernameRegex.MatchString(s) {
		return fmt.Errorf("username may only contain letters, numbers, or underscores")
	}
	return nil
}

func ValidatePassword(s string) error {
	if len(s) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	var hasUpper, hasDigit, hasSpecial bool
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}
	return nil
}

func IsValidISBN(cleaned string) bool {
	if len(cleaned) != 10 && len(cleaned) != 13 {
		return false
	}
	for i, r := range cleaned {
		if r >= '0' && r <= '9' {
			continue
		}
		if i == 9 && len(cleaned) == 10 && (r == 'X' || r == 'x') {
			continue
		}
		return false
	}
	return true
}

func normalizeFreeText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// generateBaseUsername derives a starting username from a patron's full
// name. Rules: split on whitespace, take the first rune of the first
// word and concatenate the last word; lowercase; strip any character
// outside [a-z0-9]. Single-word names fall through to the whole name
// lowercased (e.g. "Cher" -> "cher"). Empty or all-whitespace input
// returns "" so callers can reject via validation rather than creating
// a row with an empty username. The returned value is a *base*: the
// caller (CreatePatron) checks the users table for collisions inside
// its transaction and appends "2", "3", ... until a free username is
// found, keeping the check atomic with the insert.
// generateTempPassword excludes visually ambiguous characters (0/O,
// 1/l/I) so credential sheets can be read aloud and hand-typed.
func generateTempPassword() (string, error) {
	const (
		uppers   = "ABCDEFGHJKMNPQRSTUVWXYZ"
		lowers   = "abcdefghjkmnpqrstuvwxyz"
		digits   = "23456789"
		specials = "!@#$%^&*"
		mixed    = uppers + lowers + digits + specials
	)
	pick := func(set string) (byte, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
		if err != nil {
			return 0, err
		}
		return set[n.Int64()], nil
	}
	out := make([]byte, 12)
	classChars := []string{uppers, digits, specials}
	for i, set := range classChars {
		c, err := pick(set)
		if err != nil {
			return "", err
		}
		out[i] = c
	}
	for i := len(classChars); i < len(out); i++ {
		c, err := pick(mixed)
		if err != nil {
			return "", err
		}
		out[i] = c
	}
	for i := len(out) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		out[i], out[j.Int64()] = out[j.Int64()], out[i]
	}
	return string(out), nil
}

func generateBaseUsername(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	if len(fields) == 1 {
		b.WriteString(fields[0])
	} else {
		firstInitial := []rune(fields[0])[0]
		b.WriteRune(firstInitial)
		b.WriteString(fields[len(fields)-1])
	}
	raw := strings.ToLower(b.String())
	var cleaned strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cleaned.WriteRune(r)
		}
	}
	return cleaned.String()
}
