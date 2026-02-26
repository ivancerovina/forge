package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ivancerovina/forge/internal/ui"
)

func ProjectListHelp() {
	fmt.Println(ui.TitleStyle.Render("forge project list") + ui.DescStyle.Render(" - list registered projects"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project list") + "  " + ui.DescStyle.Render("Show all registered projects"))
}

func ProjectList(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			ProjectListHelp()
			os.Exit(0)
		}
	}

	paths, err := readProjects()
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("Could not read projects: "+err.Error()))
		os.Exit(1)
	}

	if len(paths) == 0 {
		fmt.Println(ui.DescStyle.Render("No projects registered."))
		return
	}

	fmt.Println(ui.HeadingStyle.Render("Registered projects:"))
	fmt.Println()

	for _, p := range paths {
		rcPath := filepath.Join(p, ".forgerc.json")
		data, err := os.ReadFile(rcPath)
		if err != nil {
			fmt.Println("  " + ui.ErrStyle.Render(p) + ui.DescStyle.Render(" (missing)"))
			continue
		}

		var project struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &project) != nil || project.Name == "" {
			fmt.Println("  " + ui.ErrStyle.Render(p) + ui.DescStyle.Render(" (invalid .forgerc.json)"))
			continue
		}

		fmt.Println("  " + ui.TitleStyle.Render(project.Name) + " " + ui.DescStyle.Render(p))
	}
}
