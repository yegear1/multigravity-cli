package config

import (
	"testing"
)

func TestValidateProfileName(t *testing.T) {
	validNames := []string{
		"default",
		"work",
		"project-1",
		"client-acme-2026",
		"ProfileA",
		"p",
	}

	for _, name := range validNames {
		if err := ValidateProfileName(name); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}

	invalidNames := []string{
		"",
		"-startwithdash",
		"with spaces",
		"with_underscores",
		"special!char",
		"name.with.dots",
	}

	for _, name := range invalidNames {
		if err := ValidateProfileName(name); err == nil {
			t.Errorf("expected %q to be invalid, but got no error", name)
		}
	}
}
