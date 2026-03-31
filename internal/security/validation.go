package security

import (
	"errors"
	"net/mail"
	"regexp"
)

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

// usernamePattern validates usernames: 3-64 chars, letters, numbers, underscores, hyphens
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)

func ValidateIdentifier(value string) error {
	if value == "" {
		return errors.New("identifier is required")
	}
	if !idPattern.MatchString(value) {
		return errors.New("identifier contains invalid characters")
	}
	return nil
}

// ValidateEmail checks that an email address is present and has a valid format.
func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return errors.New("invalid email format")
	}
	return nil
}

// ValidateUsername checks that a username is present and matches the allowed pattern.
func ValidateUsername(username string) error {
	if username == "" {
		return errors.New("username is required")
	}
	if len(username) < 3 || len(username) > 64 {
		return errors.New("username must be 3-64 characters")
	}
	if !usernamePattern.MatchString(username) {
		return errors.New("username can only contain letters, numbers, underscores, and hyphens")
	}
	return nil
}
