package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ivancerovina/forge/internal/ui"
)

func Help() {
	fmt.Println(ui.TitleStyle.Render("forge") + ui.DescStyle.Render(" - project management CLI for developers"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge") + " " + ui.DescStyle.Render("<command> [arguments]"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Commands:"))
	fmt.Println("  " + ui.CmdStyle.Render("init") + "        " + ui.DescStyle.Render("Initialize forge system (Traefik, Docker network)"))
	fmt.Println("  " + ui.CmdStyle.Render("project") + "     " + ui.DescStyle.Render("Manage forge projects (init, register, unregister, list)"))
	fmt.Println("  " + ui.CmdStyle.Render("start") + "       " + ui.DescStyle.Render("Start the project environment"))
	fmt.Println("  " + ui.CmdStyle.Render("stop") + "        " + ui.DescStyle.Render("Stop the project environment"))
	fmt.Println("  " + ui.CmdStyle.Render("destroy") + "     " + ui.DescStyle.Render("Destroy the project environment"))
	fmt.Println("  " + ui.CmdStyle.Render("help") + "        " + ui.DescStyle.Render("Show this help message"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))

	if data, err := os.ReadFile(".forgerc.json"); err == nil {
		var project struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &project) == nil && project.Name != "" {
			fmt.Println()
			fmt.Println(ui.HeadingStyle.Render("Current project: ") + ui.CmdStyle.Render(project.Name))
		}
	}
}
