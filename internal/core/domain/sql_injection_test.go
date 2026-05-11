package domain

import (
	"errors"
	"testing"
)

func TestValidateFunnyNameForSQLInjection_ValidNames(t *testing.T) {
	validNames := []string{"Mickey", "DONALD", "BARNACLE", "Pickles", "Scooby", "Jerry", "Tom", "Bugs", "Daffy"}
	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFunnyNameForSQLInjection(name); err != nil {
				t.Errorf("ValidateFunnyNameForSQLInjection(%q) returned error: %v", name, err)
			}
		})
	}
}

func TestValidateFunnyNameForSQLInjection_InvalidEmpty(t *testing.T) {
	err := ValidateFunnyNameForSQLInjection("")
	if !errors.Is(err, ErrInvalidFunnyName) {
		t.Errorf("ValidateFunnyNameForSQLInjection(%q) = %v, want ErrInvalidFunnyName", "", err)
	}
}

func TestValidateFunnyNameForSQLInjection_InvalidCharacters(t *testing.T) {
	invalidCases := []struct {
		name string
	}{
		{"Mickey123"},
		{"123Bugs"},
		{"Mickey-1"},
		{"Tom & Jerry"},
		{"Scooby!"},
		{"Daffy?"},
		{"Mickey "},
		{" Mickey"},
		{"Mickey\t"},
		{"Robert'); DROP TABLE Students;--"},
		{"Mickey' OR '1'='1"},
		{"Mickey\"; DELETE FROM users WHERE 1=1; --"},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFunnyNameForSQLInjection(tc.name)
			if err == nil {
				t.Errorf("ValidateFunnyNameForSQLInjection(%q) = nil, expected error for SQL injection attempt", tc.name)
			}
		})
	}
}

func TestValidateFunnyNameForSQLInjection_TooShort(t *testing.T) {
	err := ValidateFunnyNameForSQLInjection("ab")
	if !errors.Is(err, ErrFunnyNameTooShort) {
		t.Errorf("ValidateFunnyNameForSQLInjection(%q) = %v, want ErrFunnyNameTooShort", "ab", err)
	}
}

func TestValidateFunnyNameForSQLInjection_TooLong(t *testing.T) {
	longName := ""
	for i := 0; i < MaxFunnyNameLength+1; i++ {
		longName += "A"
	}
	err := ValidateFunnyNameForSQLInjection(longName)
	if !errors.Is(err, ErrFunnyNameTooLong) {
		t.Errorf("ValidateFunnyNameForSQLInjection(longName) = %v, want ErrFunnyNameTooLong", err)
	}
}

func TestValidateFunnyNameForSQLInjection_NotInCuratedList(t *testing.T) {
	notInList := []string{"Basuruk", "OmniView", "Random", "Xyz", "TestName"}
	for _, name := range notInList {
		t.Run(name, func(t *testing.T) {
			err := ValidateFunnyNameForSQLInjection(name)
			if err == nil {
				t.Errorf("ValidateFunnyNameForSQLInjection(%q) = nil, expected error (not in curated list)", name)
			}
		})
	}
}

func TestIsFunnyNameSQLSafe(t *testing.T) {
	tests := []struct {
		name  string
		isSQL bool
	}{
		{"Mickey", true},
		{"BARNACLE", true},
		{"Mickey_Mouse", false},
		{"", false},
		{"Mickey123", false},
		{"Mickey ", false},
		{"Mickey'; DROP TABLE--", false},
		{"Tom & Jerry", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsFunnyNameSQLSafe(tc.name)
			if got != tc.isSQL {
				t.Errorf("IsFunnyNameSQLSafe(%q) = %v, want %v", tc.name, got, tc.isSQL)
			}
		})
	}
}

func TestNormalizeFunnyNameForSQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Mickey", "MICKEY"},
		{"mickey", "MICKEY"},
		{"MiCkEy", "MICKEY"},
		{"  Mickey  ", "MICKEY"},
		{"barnacle", "BARNACLE"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := NormalizeFunnyNameForSQL(tc.input)
			if got != tc.expected {
				t.Errorf("NormalizeFunnyNameForSQL(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}