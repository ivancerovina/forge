package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type EnvironmentCommands struct {
	Start   []string `json:"start"`
	Stop    []string `json:"stop"`
	Destroy []string `json:"destroy"`
}

type AliasEntry struct {
	Port  int     `json:"port"`
	Alias *string `json:"alias"`          // nil = index (just <code>.local), string = <alias>.<code>.local
	HTTPS *bool   `json:"https,omitempty"` // nil/true = HTTPS, false = HTTP only
}

type Hooks struct {
	PreStart    []string `json:"pre_start,omitempty"`
	PostStart   []string `json:"post_start,omitempty"`
	PreStop     []string `json:"pre_stop,omitempty"`
	PostStop    []string `json:"post_stop,omitempty"`
	PreDestroy  []string `json:"pre_destroy,omitempty"`
	PostDestroy []string `json:"post_destroy,omitempty"`
}

type Environment struct {
	ComposeFile string                `json:"compose_file,omitempty"`
	Hooks       Hooks                 `json:"hooks,omitempty"`
	Alias       map[string]AliasEntry `json:"alias"`
	Commands    *EnvironmentCommands  `json:"commands,omitempty"` // legacy, nil when absent
}

// IsLegacy returns true if the environment uses the legacy "commands" format.
func (e Environment) IsLegacy() bool { return e.Commands != nil }

type ForgeProject struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Code        string      `json:"code"`
	Environment Environment `json:"environment"`
}

// UserHomeDir returns the home directory of the current user.
func UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

// ForgeDir returns the path to ~/.forge.
func ForgeDir() (string, error) {
	home, err := UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".forge"), nil
}

// ReadForgeRC reads and parses .forgerc.json from the given directory.
func ReadForgeRC(dir string) (ForgeProject, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".forgerc.json"))
	if err != nil {
		return ForgeProject{}, err
	}
	var p ForgeProject
	if err := json.Unmarshal(data, &p); err != nil {
		return ForgeProject{}, err
	}
	return p, nil
}

func projectsFilePath() (string, error) {
	forgeDir, err := ForgeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(forgeDir, "projects.json"), nil
}

// ReadProjects reads the list of registered project paths from ~/.forge/projects.json.
func ReadProjects() ([]string, error) {
	path, err := projectsFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var paths []string
	if err := json.Unmarshal(data, &paths); err != nil {
		return nil, err
	}
	return paths, nil
}

// WriteProjects writes the list of registered project paths to ~/.forge/projects.json.
func WriteProjects(paths []string) error {
	fp, err := projectsFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fp, append(data, '\n'), 0o644)
}

// RegisterProject registers the given directory in ~/.forge/projects.json.
// Returns true if newly registered, false if already registered.
func RegisterProject(dir string) (bool, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false, fmt.Errorf("could not resolve path: %w", err)
	}

	paths, err := ReadProjects()
	if err != nil {
		return false, fmt.Errorf("could not read projects list: %w", err)
	}

	for _, p := range paths {
		if p == abs {
			return false, nil
		}
	}

	paths = append(paths, abs)
	if err := WriteProjects(paths); err != nil {
		return false, fmt.Errorf("could not write projects list: %w", err)
	}
	return true, nil
}

// UnregisterProject removes the given directory from ~/.forge/projects.json.
// Returns true if removed, false if not found.
func UnregisterProject(dir string) (bool, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false, fmt.Errorf("could not resolve path: %w", err)
	}

	paths, err := ReadProjects()
	if err != nil {
		return false, fmt.Errorf("could not read projects list: %w", err)
	}

	found := false
	filtered := paths[:0]
	for _, p := range paths {
		if p == abs {
			found = true
			continue
		}
		filtered = append(filtered, p)
	}

	if !found {
		return false, nil
	}

	if err := WriteProjects(filtered); err != nil {
		return false, fmt.Errorf("could not write projects list: %w", err)
	}
	return true, nil
}

// ResolveProjectPath resolves a path flag or positional arg to an absolute path.
// Falls back to cwd if neither is provided.
func ResolveProjectPath(pathFlag string, positionalArgs []string) (string, error) {
	dir := pathFlag
	if dir == "" && len(positionalArgs) > 0 {
		dir = positionalArgs[0]
	}
	if dir == "" {
		dir = "."
	}
	return filepath.Abs(dir)
}
