package security

import (
	"errors"
	"regexp"
)

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

func ValidateIdentifier(value string) error {
	if value == "" {
		return errors.New("identifier is required")
	}
	if !idPattern.MatchString(value) {
		return errors.New("identifier contains invalid characters")
	}
	return nil
}
