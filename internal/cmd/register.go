package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ivancerovina/forge/internal/ui"
)

type EnvironmentCommands struct {
	Start   []string `json:"start"`
	Stop    []string `json:"stop"`
	Destroy []string `json:"destroy"`
}

type AliasEntry struct {
	Port  int     `json:"port"`
	Alias *string `json:"alias"`          // nil = index (just <project-id>.local), string = <alias>.<project-id>.local
	HTTPS *bool   `json:"https,omitempty"` // nil/true = HTTPS, false = HTTP only
}

type Environment struct {
	Commands EnvironmentCommands    `json:"commands"`
	Alias    map[string]AliasEntry  `json:"alias"`
}

type ForgeProject struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Code        string      `json:"code"`
	Environment Environment `json:"environment"`
}

func readForgeRCAt(dir string) (ForgeProject, error) {
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
	home, err := UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".forge", "projects.json"), nil
}

func readProjects() ([]string, error) {
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

func writeProjects(paths []string) error {
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

// resolveProjectPath resolves the --path flag or positional arg to an absolute path.
// Falls back to cwd if neither is provided.
func resolveProjectPath(pathFlag string, positionalArgs []string) (string, error) {
	dir := pathFlag
	if dir == "" && len(positionalArgs) > 0 {
		dir = positionalArgs[0]
	}
	if dir == "" {
		dir = "."
	}
	return filepath.Abs(dir)
}

// RegisterProject registers the given directory in ~/.forge/projects.json.
// Returns true if newly registered, false if already registered.
func RegisterProject(dir string) (bool, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false, fmt.Errorf("could not resolve path: %w", err)
	}

	paths, err := readProjects()
	if err != nil {
		return false, fmt.Errorf("could not read projects list: %w", err)
	}

	for _, p := range paths {
		if p == abs {
			return false, nil
		}
	}

	paths = append(paths, abs)
	if err := writeProjects(paths); err != nil {
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

	paths, err := readProjects()
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

	if err := writeProjects(filtered); err != nil {
		return false, fmt.Errorf("could not write projects list: %w", err)
	}
	return true, nil
}

func RegisterHelp() {
	fmt.Println(ui.TitleStyle.Render("forge project register") + ui.DescStyle.Render(" - register a project"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project register") + "          " + ui.DescStyle.Render("Register the project in the current directory"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project register -p <path>") + " " + ui.DescStyle.Render("Register the project at the given path"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("-p, --path") + "  " + ui.DescStyle.Render("Path to the project directory (defaults to cwd)"))
	fmt.Println("  " + ui.CmdStyle.Render("    --help") + "  " + ui.DescStyle.Render("Show this help message"))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Requires a .forgerc.json file. Run ") + ui.CmdStyle.Render("forge project init") + ui.DescStyle.Render(" first."))
}

func Register(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			RegisterHelp()
			os.Exit(0)
		}
	}

	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	pathFlag := fs.String("path", "", "Path to the project directory")
	fs.StringVar(pathFlag, "p", "", "Path to the project directory (shorthand)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	dir, err := resolveProjectPath(*pathFlag, fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	if _, err := readForgeRCAt(dir); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No .forgerc.json found at "+dir+".")+
			" Run "+ui.CmdStyle.Render("forge project init")+" first.")
		os.Exit(1)
	}

	added, err := RegisterProject(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	if added {
		fmt.Println(ui.TitleStyle.Render("Project registered!"))
	} else {
		fmt.Println(ui.DescStyle.Render("Project is already registered."))
	}
}

func UnregisterHelp() {
	fmt.Println(ui.TitleStyle.Render("forge project unregister") + ui.DescStyle.Render(" - unregister a project"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project unregister") + "          " + ui.DescStyle.Render("Unregister the project in the current directory"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project unregister -p <path>") + " " + ui.DescStyle.Render("Unregister the project at the given path"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("-p, --path") + "  " + ui.DescStyle.Render("Path to the project directory (defaults to cwd)"))
	fmt.Println("  " + ui.CmdStyle.Render("    --help") + "  " + ui.DescStyle.Render("Show this help message"))
}

func Unregister(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			UnregisterHelp()
			os.Exit(0)
		}
	}

	fs := flag.NewFlagSet("unregister", flag.ContinueOnError)
	pathFlag := fs.String("path", "", "Path to the project directory")
	fs.StringVar(pathFlag, "p", "", "Path to the project directory (shorthand)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	dir, err := resolveProjectPath(*pathFlag, fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	removed, err := UnregisterProject(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	if removed {
		fmt.Println(ui.TitleStyle.Render("Project unregistered!"))
	} else {
		fmt.Println(ui.DescStyle.Render("Project is not registered."))
	}
}
