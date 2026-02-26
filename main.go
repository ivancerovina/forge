package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ivancerovina/forge/internal/cmd"
	"github.com/ivancerovina/forge/internal/ui"
)

func initForge() error {
	home, err := cmd.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	forgeDir := filepath.Join(home, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", forgeDir, err)
	}

	traefikDir := filepath.Join(forgeDir, "traefik")
	if err := os.MkdirAll(traefikDir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", traefikDir, err)
	}

	certsDir := filepath.Join(forgeDir, "certs")
	if err := os.MkdirAll(certsDir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", certsDir, err)
	}

	files := map[string]string{
		"config.json":   "{}",
		"projects.json": "[]",
	}

	for name, content := range files {
		path := filepath.Join(forgeDir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
			return fmt.Errorf("could not create %s: %w", path, err)
		}
	}

	return nil
}

func main() {
	if err := initForge(); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	if len(os.Args) < 2 || os.Args[1] == "--help" || os.Args[1] == "help" {
		cmd.Help()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "init":
		cmd.SystemInit(os.Args[2:])
	case "project":
		cmd.Project(os.Args[2:])
	case "start":
		cmd.Start(os.Args[2:])
	case "stop":
		cmd.Stop(os.Args[2:])
	case "destroy":
		cmd.Destroy(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "%s %s\n", ui.ErrStyle.Render("unknown command:"), os.Args[1])
		os.Exit(1)
	}
}
