package config

import (
	"fmt"
	"regexp"
	"strings"
)

var pathRegexp = regexp.MustCompile(`^[a-zA-Z0-9/_-]+$`)

var CodeRegexp = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
var serviceNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

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

func ValidateServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if !serviceNameRegexp.MatchString(name) {
		return fmt.Errorf("service name must start with a letter or number and contain only letters, numbers, hyphens, and underscores")
	}
	return nil
}

func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func ValidatePath(path string) error {
	if path == "" {
		return nil
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path must start with /")
	}
	if path != "/" && strings.HasSuffix(path, "/") {
		return fmt.Errorf("path must not end with /")
	}
	if !pathRegexp.MatchString(path) {
		return fmt.Errorf("path may only contain letters, numbers, slashes, hyphens, and underscores")
	}
	return nil
}

// ValidateDomain validates a domain name (e.g. "dev.example.com").
func ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return fmt.Errorf("domain must have at least two parts (e.g. dev.example.com)")
	}
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("domain contains empty label")
		}
		if !CodeRegexp.MatchString(part) {
			return fmt.Errorf("domain label %q must contain only letters, numbers, and hyphens", part)
		}
	}
	return nil
}

func ValidateAliasSubdomain(alias string) error {
	if alias == "" {
		return nil
	}
	if !CodeRegexp.MatchString(alias) {
		return fmt.Errorf("alias subdomain must contain only letters, numbers, and hyphens, and cannot start or end with a hyphen")
	}
	return nil
}
