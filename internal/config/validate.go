package config

import (
	"fmt"
	"regexp"
	"strings"
)

var CodeRegexp = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if strings.Contains(name, "\n") {
		return fmt.Errorf("project name must be a single line")
	}
	return nil
}

func ValidateCode(code string) error {
	if !CodeRegexp.MatchString(code) {
		return fmt.Errorf("code must contain only letters, numbers, and hyphens, and cannot start or end with a hyphen")
	}
	return nil
}
