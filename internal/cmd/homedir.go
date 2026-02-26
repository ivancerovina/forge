package cmd

import (
	"fmt"
	"os"
	"os/user"
)

// UserHomeDir returns the home directory of the real user, even when running
// under sudo. When SUDO_USER is set, it looks up that user's home directory
// instead of relying on os.UserHomeDir() (which returns /root under sudo).
func UserHomeDir() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		u, err := user.Lookup(sudoUser)
		if err != nil {
			return "", fmt.Errorf("could not look up SUDO_USER %q: %w", sudoUser, err)
		}
		return u.HomeDir, nil
	}
	return os.UserHomeDir()
}
