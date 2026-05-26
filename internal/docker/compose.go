package docker

import (
	"fmt"
	"os"
	"path/filepath"
)

var ComposeFileCandidates = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yml",
	"docker-compose.yaml",
}

// ResolveComposeFile returns the path to the compose file for a project.
// If explicitFile is set, it is joined with projectDir and verified.
// Otherwise, it probes ComposeFileCandidates in order.
func ResolveComposeFile(projectDir, explicitFile string) (string, error) {
	if explicitFile != "" {
		path := filepath.Join(projectDir, explicitFile)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("compose file not found: %s", path)
		}
		return path, nil
	}

	for _, name := range ComposeFileCandidates {
		path := filepath.Join(projectDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no compose file found in %s (tried %v)", projectDir, ComposeFileCandidates)
}
