package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ivancerovina/forge/internal/ui"
)

func runCommands(commands []string) error {
	for _, c := range commands {
		cmd := exec.Command("sh", "-c", c)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command %q failed: %w", c, err)
		}
	}
	return nil
}

func StartHelp() {
	fmt.Println(ui.TitleStyle.Render("forge start") + ui.DescStyle.Render(" - start the project environment"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge start") + "  " + ui.DescStyle.Render("Run the start commands from .forgerc.json"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Executes each command in ") + ui.CmdStyle.Render("environment.commands.start") + ui.DescStyle.Render(" sequentially."))
}

func Start(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			StartHelp()
			os.Exit(0)
		}
	}

	p, err := readForgeRCAt(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No .forgerc.json found in the current directory."))
		os.Exit(1)
	}

	cmds := p.Environment.Commands.Start
	if len(cmds) == 0 {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No start commands defined in .forgerc.json."))
		os.Exit(1)
	}

	if err := runCommands(cmds); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}
}

func StopHelp() {
	fmt.Println(ui.TitleStyle.Render("forge stop") + ui.DescStyle.Render(" - stop the project environment"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge stop") + "  " + ui.DescStyle.Render("Run the stop commands from .forgerc.json"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Executes each command in ") + ui.CmdStyle.Render("environment.commands.stop") + ui.DescStyle.Render(" sequentially."))
}

func Stop(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			StopHelp()
			os.Exit(0)
		}
	}

	p, err := readForgeRCAt(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No .forgerc.json found in the current directory."))
		os.Exit(1)
	}

	cmds := p.Environment.Commands.Stop
	if len(cmds) == 0 {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No stop commands defined in .forgerc.json."))
		os.Exit(1)
	}

	if err := runCommands(cmds); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}
}

func DestroyHelp() {
	fmt.Println(ui.TitleStyle.Render("forge destroy") + ui.DescStyle.Render(" - destroy the project environment"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge destroy") + "  " + ui.DescStyle.Render("Run the destroy commands from .forgerc.json"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Executes each command in ") + ui.CmdStyle.Render("environment.commands.destroy") + ui.DescStyle.Render(" sequentially."))
}

func Destroy(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			DestroyHelp()
			os.Exit(0)
		}
	}

	p, err := readForgeRCAt(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No .forgerc.json found in the current directory."))
		os.Exit(1)
	}

	cmds := p.Environment.Commands.Destroy
	if len(cmds) == 0 {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No destroy commands defined in .forgerc.json."))
		os.Exit(1)
	}

	if err := runCommands(cmds); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}
}
