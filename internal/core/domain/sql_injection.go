package domain

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	sqlInjectionPreventionPattern = `^[A-Za-z_]+$`
)

var safeNameRegex = regexp.MustCompile(sqlInjectionPreventionPattern)

// ValidateFunnyNameForSQLInjection ensures a funny name is safe for SQL use.
func ValidateFunnyNameForSQLInjection(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidFunnyName)
	}
	if len(name) < MinFunnyNameLength {
		return fmt.Errorf("%w: %q is too short (min %d chars)", ErrFunnyNameTooShort, name, MinFunnyNameLength)
	}
	if len(name) > MaxFunnyNameLength {
		return fmt.Errorf("%w: %q exceeds max length (%d chars)", ErrFunnyNameTooLong, name, MaxFunnyNameLength)
	}
	if !safeNameRegex.MatchString(name) {
		return fmt.Errorf("%w: %q contains invalid characters (only letters and underscores allowed)", ErrInvalidFunnyName, name)
	}
	if !isFunnyNameInList(name) {
		return fmt.Errorf("%w: %q is not in the curated funny name list", ErrInvalidFunnyName, name)
	}
	return nil
}

// IsFunnyNameSQLSafe reports whether the funny name is safe for SQL use.
func IsFunnyNameSQLSafe(name string) bool {
	return ValidateFunnyNameForSQLInjection(name) == nil
}

// NormalizeFunnyNameForSQL normalizes a funny name to uppercase for SQL use.
func NormalizeFunnyNameForSQL(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}
