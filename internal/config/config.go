package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EnvironmentCommands struct {
	Start   []string `json:"start"`
	Stop    []string `json:"stop"`
	Destroy []string `json:"destroy"`
}

type AliasEntry struct {
	Port       int     `json:"port"`
	Alias      *string `json:"alias"`              // nil = index (just <code>.local), string = <alias>.<code>.local
	Path       string  `json:"path,omitempty"`     // e.g. "/api" — path prefix for Traefik routing
	HTTPS      *bool   `json:"https,omitempty"`    // nil/true = HTTPS, false = HTTP only
	Cloudflare *bool   `json:"cloudflare,omitempty"` // nil/false = local only, true = also bind via CF tunnel
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
	Schema      string      `json:"$schema,omitempty"`
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

// ProjectLocation holds a parsed ForgeProject and the directory where .forgerc.json was found.
type ProjectLocation struct {
	Project ForgeProject
	Dir     string // absolute path to directory containing .forgerc.json
}

// FindForgeRC walks up from startDir looking for .forgerc.json.
// Stops at: (1) directory containing .forgerc.json, (2) directory containing .git
// (without .forgerc.json), (3) user home directory, (4) filesystem root.
func FindForgeRC(startDir string) (ProjectLocation, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return ProjectLocation{}, fmt.Errorf("could not resolve path: %w", err)
	}

	home, _ := os.UserHomeDir()

	for {
		// Check for .forgerc.json first (it commonly coexists with .git)
		rcPath := filepath.Join(dir, ".forgerc.json")
		if _, statErr := os.Stat(rcPath); statErr == nil {
			// File exists — must parse successfully or it's a real error
			p, err := ReadForgeRC(dir)
			if err != nil {
				return ProjectLocation{}, fmt.Errorf("found .forgerc.json in %s but it is invalid: %w", dir, err)
			}
			return ProjectLocation{Project: p, Dir: dir}, nil
		}

		// If .git exists here but no .forgerc.json, stop searching
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return ProjectLocation{}, fmt.Errorf("no .forgerc.json found (searched up to git root %s)", dir)
		}

		// Stop at home directory
		if dir == home {
			return ProjectLocation{}, fmt.Errorf("no .forgerc.json found (searched up to home directory)")
		}

		// Move to parent
		parent := filepath.Dir(dir)
		if parent == dir {
			// Filesystem root
			return ProjectLocation{}, fmt.Errorf("no .forgerc.json found")
		}
		dir = parent
	}
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

// FindRegisteredProject looks up a registered project by name (case-insensitive).
// It reads all registered project paths, loads each .forgerc.json, and returns
// the first match.
func FindRegisteredProject(name string) (ProjectLocation, error) {
	paths, err := ReadProjects()
	if err != nil {
		return ProjectLocation{}, fmt.Errorf("could not read projects list: %w", err)
	}

	for _, p := range paths {
		project, err := ReadForgeRC(p)
		if err != nil {
			continue
		}
		if strings.EqualFold(project.Name, name) {
			return ProjectLocation{Project: project, Dir: p}, nil
		}
	}

	return ProjectLocation{}, fmt.Errorf("no registered project found with name %q", name)
}

// WriteForgeRC writes the given ForgeProject as .forgerc.json in the given directory.
func WriteForgeRC(dir string, project ForgeProject) error {
	project.Schema = SchemaURI()
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".forgerc.json"), append(data, '\n'), 0o644)
}

// ForgeConfig represents the global forge configuration stored in ~/.forge/config.json.
type ForgeConfig struct {
	CloudflareDomain string `json:"cloudflare_domain,omitempty"`
	CloudflareTunnel bool   `json:"cloudflare_tunnel,omitempty"` // enables cloudflared container
}

func configFilePath() (string, error) {
	forgeDir, err := ForgeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(forgeDir, "config.json"), nil
}

// ReadConfig reads the global forge configuration from ~/.forge/config.json.
func ReadConfig() (ForgeConfig, error) {
	path, err := configFilePath()
	if err != nil {
		return ForgeConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ForgeConfig{}, err
	}
	var cfg ForgeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ForgeConfig{}, err
	}
	return cfg, nil
}

// WriteConfig writes the global forge configuration to ~/.forge/config.json.
func WriteConfig(cfg ForgeConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
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
