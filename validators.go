package main

import (
	"fmt"
	"regexp"
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
