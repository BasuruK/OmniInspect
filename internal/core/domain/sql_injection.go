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

// ValidateFunnyNameForSQLInjection checks if the provided funny name is valid and safe to use in SQL contexts, preventing SQL injection vulnerabilities.
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

// IsFunnyNameSQLSafe checks if the funny name is valid and safe for SQL usage by validating it against the defined criteria.
func IsFunnyNameSQLSafe(name string) bool {
	return ValidateFunnyNameForSQLInjection(name) == nil
}

// NormalizeFunnyNameForSQL normalizes the funny name by trimming whitespace and converting it to uppercase, ensuring consistent formatting for SQL usage.
func NormalizeFunnyNameForSQL(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}
