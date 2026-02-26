package cmd

import (
	"fmt"
	"os"

	"github.com/ivancerovina/forge/internal/ui"
)

func ProjectHelp() {
	fmt.Println(ui.TitleStyle.Render("forge project") + ui.DescStyle.Render(" - manage forge projects"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project") + " " + ui.DescStyle.Render("<command> [arguments]"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Commands:"))
	fmt.Println("  " + ui.CmdStyle.Render("init") + "          " + ui.DescStyle.Render("Initialize a new forge project"))
	fmt.Println("  " + ui.CmdStyle.Render("register") + "      " + ui.DescStyle.Render("Register a project"))
	fmt.Println("  " + ui.CmdStyle.Render("unregister") + "    " + ui.DescStyle.Render("Unregister a project"))
	fmt.Println("  " + ui.CmdStyle.Render("list") + "          " + ui.DescStyle.Render("List registered projects"))
	fmt.Println("  " + ui.CmdStyle.Render("status") + "        " + ui.DescStyle.Render("Show service connectivity status"))
	fmt.Println("  " + ui.CmdStyle.Render("bind") + "          " + ui.DescStyle.Render("Bind project domains (requires sudo)"))
	fmt.Println("  " + ui.CmdStyle.Render("unbind") + "        " + ui.DescStyle.Render("Remove project domain bindings (requires sudo)"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))
}

func Project(args []string) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		ProjectHelp()
		os.Exit(0)
	}

	switch args[0] {
	case "init":
		Init(args[1:])
	case "register":
		Register(args[1:])
	case "unregister":
		Unregister(args[1:])
	case "list":
		ProjectList(args[1:])
	case "status":
		Status(args[1:])
	case "bind":
		Bind(args[1:])
	case "unbind":
		Unbind(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "%s %s\n", ui.ErrStyle.Render("unknown project command:"), args[0])
		os.Exit(1)
	}
}
