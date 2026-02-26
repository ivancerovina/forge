package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/ivancerovina/forge/internal/ui"
)

var codeRegexp = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

func validateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if strings.Contains(name, "\n") {
		return fmt.Errorf("project name must be a single line")
	}
	return nil
}

func validateCode(code string) error {
	if !codeRegexp.MatchString(code) {
		return fmt.Errorf("code must contain only letters, numbers, and hyphens, and cannot start or end with a hyphen")
	}
	return nil
}

func isGitRepo() bool {
	_, err := os.Stat(".git")
	return err == nil
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupGitRemote(url string) error {
	if err := runGit("remote", "add", "origin", url); err != nil {
		return runGit("remote", "set-url", "origin", url)
	}
	return nil
}

func InitHelp() {
	fmt.Println(ui.TitleStyle.Render("forge project init") + ui.DescStyle.Render(" - initialize a new forge project"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project init") + "                          " + ui.DescStyle.Render("Interactive mode"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project init -t <name> -c <code>") + "      " + ui.DescStyle.Render("Non-interactive mode"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("-p, --path") + "         " + ui.DescStyle.Render("Directory to initialize in (defaults to cwd)"))
	fmt.Println("  " + ui.CmdStyle.Render("-t, --title") + "        " + ui.DescStyle.Render("Project name (required with -c)"))
	fmt.Println("  " + ui.CmdStyle.Render("-c, --code") + "         " + ui.DescStyle.Render("Project code (required with -t)"))
	fmt.Println("  " + ui.CmdStyle.Render("-d, --description") + "  " + ui.DescStyle.Render("Project description (optional)"))
	fmt.Println("  " + ui.CmdStyle.Render("-r, --remote") + "       " + ui.DescStyle.Render("Git remote URL (non-interactive; implies git init)"))
	fmt.Println("  " + ui.CmdStyle.Render("    --help") + "         " + ui.DescStyle.Render("Show this help message"))
}

func Init(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			InitHelp()
			os.Exit(0)
		}
	}

	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	pathFlag := fs.String("path", "", "Directory to initialize in")
	fs.StringVar(pathFlag, "p", "", "Directory to initialize in (shorthand)")
	title := fs.String("title", "", "Project name")
	fs.StringVar(title, "t", "", "Project name (shorthand)")
	code := fs.String("code", "", "Project code")
	fs.StringVar(code, "c", "", "Project code (shorthand)")
	desc := fs.String("description", "", "Project description")
	fs.StringVar(desc, "d", "", "Project description (shorthand)")
	remote := fs.String("remote", "", "Git remote URL")
	fs.StringVar(remote, "r", "", "Git remote URL (shorthand)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *pathFlag != "" {
		if err := os.Chdir(*pathFlag); err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("cannot change to directory: "+err.Error()))
			os.Exit(1)
		}
	}

	// Check for overwrite
	if _, err := os.Stat(".forgerc.json"); err == nil {
		var overwrite bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("A .forgerc.json already exists. Overwrite?").
					Affirmative("Yes").
					Negative("No").
					Value(&overwrite),
			),
		)
		if err := form.Run(); err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
			os.Exit(1)
		}
		if !overwrite {
			return
		}
	}

	hasTitle := *title != ""
	hasCode := *code != ""

	// If one of the required flags is provided but not the other, error
	if hasTitle != hasCode {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("both --title and --code are required"))
		os.Exit(1)
	}

	// Interactive mode
	if !hasTitle && !hasCode {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Project name").
					Value(title).
					Validate(func(s string) error { return validateName(s) }),
				huh.NewText().
					Title("Description").
					Value(desc),
				huh.NewInput().
					Title("Project code").
					Value(code).
					Validate(func(s string) error { return validateCode(s) }),
			),
		)
		if err := form.Run(); err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
			os.Exit(1)
		}
	} else {
		// Non-interactive validation
		if err := validateName(*title); err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
			os.Exit(1)
		}
		if err := validateCode(*code); err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
			os.Exit(1)
		}
	}

	project := map[string]any{
		"name":        strings.TrimSpace(*title),
		"description": *desc,
		"code":        *code,
		"environment": map[string]any{
			"commands": map[string][]string{
				"start":   {},
				"stop":    {},
				"destroy": {},
			},
			"alias": map[string]any{},
		},
	}

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	if err := os.WriteFile(".forgerc.json", append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	// Git setup
	interactive := !hasTitle && !hasCode

	if interactive {
		if isGitRepo() {
			fmt.Println(ui.DescStyle.Render("Git repository already initialized."))
		} else {
			var initGit bool
			var remoteURL string

			gitForm := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Initialize a git repository?").
						Affirmative("Yes").
						Negative("No").
						Value(&initGit),
				),
				huh.NewGroup(
					huh.NewInput().
						Title("Remote URL (optional)").
						Value(&remoteURL),
				).WithHideFunc(func() bool { return !initGit }),
			)
			if err := gitForm.Run(); err != nil {
				fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
				os.Exit(1)
			}

			if initGit {
				if err := runGit("init"); err != nil {
					fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("failed to initialize git repository: "+err.Error()))
					os.Exit(1)
				}
				if remoteURL != "" {
					if err := setupGitRemote(remoteURL); err != nil {
						fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("failed to set git remote: "+err.Error()))
						os.Exit(1)
					}
				}
			}
		}
	} else if *remote != "" {
		if !isGitRepo() {
			if err := runGit("init"); err != nil {
				fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("failed to initialize git repository: "+err.Error()))
				os.Exit(1)
			}
		}
		if err := setupGitRemote(*remote); err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("failed to set git remote: "+err.Error()))
			os.Exit(1)
		}
	}

	// Register prompt
	var register bool
	regForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Register this project?").
				Affirmative("Yes").
				Negative("No").
				Value(&register),
		),
	)
	if err := regForm.Run(); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}
	if register {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("could not determine current directory: "+err.Error()))
			os.Exit(1)
		}
		if _, err := RegisterProject(cwd); err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("failed to register project: "+err.Error()))
			os.Exit(1)
		}
	}

	fmt.Println(ui.TitleStyle.Render("Project initialized!"))
}
