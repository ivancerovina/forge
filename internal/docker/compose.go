package docker

import (
	"fmt"
	"os"
	"os/exec"
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

// ComposeUp runs docker compose up -d for the given compose file.
func ComposeUp(composeFile, projectDir string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

// ComposeUpAttached runs docker compose up (no -d) for the given compose file.
// Streams logs in the foreground until the user interrupts (Ctrl+C).
// If watch is true, adds --watch to also sync/rebuild on file changes.
func ComposeUpAttached(composeFile, projectDir string, watch bool) error {
	args := []string{"compose", "-f", composeFile, "up"}
	if watch {
		args = append(args, "--watch")
	}
	cmd := exec.Command("docker", args...)
	cmd.Dir = projectDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

// ComposeWatch runs docker compose watch for the given compose file.
// Blocks in the foreground until the user interrupts (Ctrl+C).
func ComposeWatch(composeFile, projectDir string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "watch")
	cmd.Dir = projectDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose watch failed: %w", err)
	}
	return nil
}

// ComposeStop runs docker compose stop for the given compose file.
func ComposeStop(composeFile, projectDir string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "stop")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose stop failed: %w", err)
	}
	return nil
}

// ComposeDown runs docker compose down for the given compose file.
func ComposeDown(composeFile, projectDir string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	return nil
}

// RunHooks executes shell hooks sequentially via sh -c.
// Hooks run with their working directory set to projectDir.
func RunHooks(hooks []string, projectDir string) error {
	for _, h := range hooks {
		cmd := exec.Command("sh", "-c", h)
		cmd.Dir = projectDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook %q failed: %w", h, err)
		}
	}
	return nil
}
