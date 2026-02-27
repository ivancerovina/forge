package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v3"

	"github.com/ivancerovina/forge/internal/bind"
	"github.com/ivancerovina/forge/internal/config"
	"github.com/ivancerovina/forge/internal/docker"
	"github.com/ivancerovina/forge/internal/system"
	"github.com/ivancerovina/forge/internal/ui"
)

func main() {
	if os.Getuid() == 0 {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("forge must not be run as root."))
		fmt.Fprintln(os.Stderr, ui.DescStyle.Render(
			"Commands that need elevated privileges will prompt for your password."))
		os.Exit(1)
	}

	root := &cli.Command{
		Name:  "forge",
		Usage: "project management CLI for developers",
		Commands: []*cli.Command{
			systemInitCmd(),
			projectCmd(),
			startCmd(),
			stopCmd(),
			destroyCmd(),
		},
	}
	if err := root.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}
}

// ensureForgeDir creates ~/.forge and its subdirectories if they don't exist.
func ensureForgeDir() error {
	forgeDir, err := config.ForgeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

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

// withInit wraps an action with forge directory initialization.
func withInit(action cli.ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		if err := ensureForgeDir(); err != nil {
			return err
		}
		return action(ctx, cmd)
	}
}

// --- Display helpers ---

func printInitStatus(name string, ok bool) {
	if ok {
		fmt.Println("  " + ui.SuccessStyle.Render("●") + " " + ui.CmdStyle.Render(name) + "  " + ui.SuccessStyle.Render("ready"))
	} else {
		fmt.Println("  " + ui.ErrStyle.Render("●") + " " + ui.CmdStyle.Render(name) + "  " + ui.ErrStyle.Render("failed"))
	}
}

func stateIndicator(state string) string {
	switch state {
	case "running":
		return ui.SuccessStyle.Render("●")
	case "exited", "dead":
		return ui.ErrStyle.Render("●")
	case "restarting", "paused", "created":
		return ui.WarningStyle.Render("●")
	default:
		return ui.DescStyle.Render("○")
	}
}

func stateLabel(state string) string {
	switch state {
	case "running":
		return ui.SuccessStyle.Render(state)
	case "exited", "dead":
		return ui.ErrStyle.Render(state)
	case "restarting", "paused", "created":
		return ui.WarningStyle.Render(state)
	default:
		return ui.DescStyle.Render(state)
	}
}

func displayServiceStatus(statuses []docker.ServiceStatus) {
	maxLen := 0
	for _, s := range statuses {
		if len(s.Name) > maxLen {
			maxLen = len(s.Name)
		}
	}

	fmt.Println(ui.HeadingStyle.Render("Services:"))
	fmt.Println()
	for _, s := range statuses {
		pad := maxLen - len(s.Name) + 2

		var connIcon string
		if s.Connected {
			connIcon = ui.TitleStyle.Render("✓")
		} else {
			connIcon = ui.DescStyle.Render("–")
		}

		var statusText string
		var indicator string
		if s.State != "" {
			indicator = stateIndicator(s.State)
			statusText = stateLabel(s.State)
			if s.Health != "" {
				statusText += ui.DescStyle.Render(" (" + s.Health + ")")
			}
		} else {
			indicator = stateIndicator("")
			statusText = ui.DescStyle.Render("not created")
		}

		fmt.Printf("  %s %s %s%*s%s\n",
			indicator,
			connIcon,
			ui.CmdStyle.Render(s.Name),
			pad, "",
			statusText)
	}
}

// --- Commands ---

func systemInitCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize forge system (Traefik, Docker network)",
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			fmt.Println(ui.TitleStyle.Render("Initializing forge system..."))
			fmt.Println()

			result, err := system.Init()
			if err != nil {
				return err
			}

			// Print step details
			for _, step := range result.Steps {
				if step.OK {
					fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(step.Name) + " " + ui.DescStyle.Render(step.Message))
				} else {
					fmt.Println("  " + ui.ErrStyle.Render("✗") + " " + ui.CmdStyle.Render(step.Name) + " " + ui.ErrStyle.Render(step.Message))
				}
			}

			// Summary
			fmt.Println()
			fmt.Println(ui.HeadingStyle.Render("Summary:"))
			anyFailed := false
			for _, step := range result.Steps {
				printInitStatus(step.Name, step.OK)
				if !step.OK {
					anyFailed = true
				}
			}

			if anyFailed {
				return fmt.Errorf("some initialization steps failed")
			}
			return nil
		}),
	}
}

func projectCmd() *cli.Command {
	return &cli.Command{
		Name:  "project",
		Usage: "Manage forge projects",
		Commands: []*cli.Command{
			projectInitCmd(),
			registerCmd(),
			unregisterCmd(),
			projectListCmd(),
			projectStatusCmd(),
			projectBindCmd(),
			projectUnbindCmd(),
		},
	}
}

func projectInitCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize a new forge project",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "path", Aliases: []string{"p"}, Usage: "Directory to initialize in"},
			&cli.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Project name (required with -c)"},
			&cli.StringFlag{Name: "code", Aliases: []string{"c"}, Usage: "Project code (required with -t)"},
			&cli.StringFlag{Name: "description", Aliases: []string{"d"}, Usage: "Project description"},
			&cli.StringFlag{Name: "remote", Aliases: []string{"r"}, Usage: "Git remote URL (implies git init)"},
			&cli.BoolFlag{Name: "register", Usage: "Register project after init"},
			&cli.BoolFlag{Name: "no-register", Usage: "Skip registration prompt"},
			&cli.BoolFlag{Name: "force", Usage: "Overwrite existing .forgerc.json without prompt"},
		},
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			pathFlag := cmd.String("path")
			title := cmd.String("title")
			code := cmd.String("code")
			desc := cmd.String("description")
			remote := cmd.String("remote")
			forceFlag := cmd.Bool("force")
			registerFlag := cmd.Bool("register")
			noRegisterFlag := cmd.Bool("no-register")

			if pathFlag != "" {
				if err := os.Chdir(pathFlag); err != nil {
					return fmt.Errorf("cannot change to directory: %w", err)
				}
			}

			hasTitle := title != ""
			hasCode := code != ""
			interactive := !hasTitle && !hasCode

			// If one of the required flags is provided but not the other, error
			if hasTitle != hasCode {
				return fmt.Errorf("both --title and --code are required")
			}

			// Check for overwrite
			if _, err := os.Stat(".forgerc.json"); err == nil {
				if interactive {
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
						return err
					}
					if !overwrite {
						return nil
					}
				} else if !forceFlag {
					return fmt.Errorf(".forgerc.json already exists (use --force to overwrite)")
				}
			}

			if interactive {
				// Interactive mode
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Project name").
							Value(&title).
							Validate(func(s string) error { return config.ValidateName(s) }),
						huh.NewText().
							Title("Description").
							Value(&desc),
						huh.NewInput().
							Title("Project code").
							Value(&code).
							Validate(func(s string) error { return config.ValidateCode(s) }),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}
			} else {
				// Non-interactive validation
				if err := config.ValidateName(title); err != nil {
					return err
				}
				if err := config.ValidateCode(code); err != nil {
					return err
				}
			}

			project := config.ForgeProject{
				Name:        strings.TrimSpace(title),
				Description: desc,
				Code:        code,
				Environment: config.Environment{
					Hooks: config.Hooks{},
					Alias: map[string]config.AliasEntry{},
				},
			}

			data, err := json.MarshalIndent(project, "", "  ")
			if err != nil {
				return err
			}

			if err := os.WriteFile(".forgerc.json", append(data, '\n'), 0o644); err != nil {
				return err
			}

			// Git setup
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
						return err
					}

					if initGit {
						if err := runGit("init"); err != nil {
							return fmt.Errorf("failed to initialize git repository: %w", err)
						}
						if remoteURL != "" {
							if err := setupGitRemote(remoteURL); err != nil {
								return fmt.Errorf("failed to set git remote: %w", err)
							}
						}
					}
				}
			} else if remote != "" {
				if !isGitRepo() {
					if err := runGit("init"); err != nil {
						return fmt.Errorf("failed to initialize git repository: %w", err)
					}
				}
				if err := setupGitRemote(remote); err != nil {
					return fmt.Errorf("failed to set git remote: %w", err)
				}
			}

			// Registration
			if interactive && !noRegisterFlag {
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
					return err
				}
				if register {
					cwd, err := os.Getwd()
					if err != nil {
						return fmt.Errorf("could not determine current directory: %w", err)
					}
					if _, err := config.RegisterProject(cwd); err != nil {
						return fmt.Errorf("failed to register project: %w", err)
					}
				}
			} else if registerFlag {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("could not determine current directory: %w", err)
				}
				if _, err := config.RegisterProject(cwd); err != nil {
					return fmt.Errorf("failed to register project: %w", err)
				}
			}

			fmt.Println(ui.TitleStyle.Render("Project initialized!"))
			return nil
		}),
	}
}

func registerCmd() *cli.Command {
	return &cli.Command{
		Name:  "register",
		Usage: "Register a project",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "path", Aliases: []string{"p"}, Usage: "Path to the project directory"},
		},
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			dir, err := config.ResolveProjectPath(cmd.String("path"), cmd.Args().Slice())
			if err != nil {
				return err
			}

			if _, err := config.ReadForgeRC(dir); err != nil {
				return fmt.Errorf("no .forgerc.json found at %s — run %s first", dir, "forge project init")
			}

			added, err := config.RegisterProject(dir)
			if err != nil {
				return err
			}

			if added {
				fmt.Println(ui.TitleStyle.Render("Project registered!"))
			} else {
				fmt.Println(ui.DescStyle.Render("Project is already registered."))
			}
			return nil
		}),
	}
}

func unregisterCmd() *cli.Command {
	return &cli.Command{
		Name:  "unregister",
		Usage: "Unregister a project",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "path", Aliases: []string{"p"}, Usage: "Path to the project directory"},
		},
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			dir, err := config.ResolveProjectPath(cmd.String("path"), cmd.Args().Slice())
			if err != nil {
				return err
			}

			removed, err := config.UnregisterProject(dir)
			if err != nil {
				return err
			}

			if removed {
				fmt.Println(ui.TitleStyle.Render("Project unregistered!"))
			} else {
				fmt.Println(ui.DescStyle.Render("Project is not registered."))
			}
			return nil
		}),
	}
}

func projectListCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List registered projects",
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			paths, err := config.ReadProjects()
			if err != nil {
				return fmt.Errorf("could not read projects: %w", err)
			}

			if len(paths) == 0 {
				fmt.Println(ui.DescStyle.Render("No projects registered."))
				return nil
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
			return nil
		}),
	}
}

func projectStatusCmd() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show service connectivity status",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var composeFile string
			var err error

			project, projErr := config.ReadForgeRC(".")
			if projErr == nil {
				composeFile, err = docker.ResolveComposeFile(".", project.Environment.ComposeFile)
			} else {
				composeFile, err = docker.ResolveComposeFile(".", "")
			}
			if err != nil {
				return fmt.Errorf("no compose file found in the current directory")
			}

			statuses, err := docker.GetServiceStatus(composeFile, ".")
			if err != nil {
				return fmt.Errorf("failed to parse compose file: %w", err)
			}

			if len(statuses) == 0 {
				return fmt.Errorf("no services found in compose file")
			}

			displayServiceStatus(statuses)
			return nil
		},
	}
}

func projectBindCmd() *cli.Command {
	return &cli.Command{
		Name:  "bind",
		Usage: "Bind project domains to local routing",
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			project, err := config.ReadForgeRC(".")
			if err != nil {
				return fmt.Errorf("no .forgerc.json found in the current directory")
			}

			if len(project.Environment.Alias) == 0 {
				return fmt.Errorf("no aliases defined in .forgerc.json — add entries to environment.alias first")
			}

			result, err := bind.Bind(project)
			if err != nil {
				return err
			}

			// Print summary
			fmt.Println(ui.TitleStyle.Render("Project domains bound!"))
			fmt.Println()
			for _, b := range result.Bindings {
				status := ui.SuccessStyle.Render("added")
				for _, e := range result.ExistingDomains {
					if e == b.Domain {
						status = ui.DescStyle.Render("already in /etc/hosts")
						break
					}
				}
				for _, w := range result.WarnedDomains {
					if w == b.Domain {
						status = ui.WarningStyle.Render("added (already in /etc/hosts from another source)")
						break
					}
				}
				scheme := "http"
				if b.HTTPS && result.HasCerts {
					scheme = "https"
				}
				fmt.Printf("  %s %s → %s\n",
					ui.SuccessStyle.Render("✓"),
					ui.CmdStyle.Render(scheme+"://"+b.Domain),
					ui.DescStyle.Render(fmt.Sprintf("%s:%d", b.Container, b.Port))+
						" "+status)
			}

			fmt.Println()
			fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(result.TraefikPath) + " " + ui.SuccessStyle.Render("written"))
			return nil
		}),
	}
}

func projectUnbindCmd() *cli.Command {
	return &cli.Command{
		Name:  "unbind",
		Usage: "Remove project domain bindings",
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			project, err := config.ReadForgeRC(".")
			if err != nil {
				return fmt.Errorf("no .forgerc.json found in the current directory")
			}

			result, err := bind.Unbind(project)
			if err != nil {
				return err
			}

			if len(result.RemovedDomains) == 0 {
				fmt.Println(ui.DescStyle.Render("No hosts entries found for project ") + ui.CmdStyle.Render(project.Code) + ui.DescStyle.Render("."))
			} else {
				fmt.Println(ui.TitleStyle.Render("Project domains unbound!"))
				fmt.Println()
				for _, domain := range result.RemovedDomains {
					fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(domain) + " " + ui.DescStyle.Render("removed from /etc/hosts"))
				}
			}

			fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(result.TraefikPath) + " " + ui.DescStyle.Render("removed"))
			return nil
		}),
	}
}

func startCmd() *cli.Command {
	return &cli.Command{
		Name:  "start",
		Usage: "Start the project environment",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			p, err := config.ReadForgeRC(".")
			if err != nil {
				return fmt.Errorf("no .forgerc.json found in the current directory")
			}

			// Legacy commands format
			if p.Environment.IsLegacy() {
				fmt.Println(ui.WarningStyle.Render("⚠ .forgerc.json uses legacy \"commands\" format. Migrate to \"hooks\" + native compose."))
				return docker.RunHooks(p.Environment.Commands.Start)
			}

			composeFile, err := docker.ResolveComposeFile(".", p.Environment.ComposeFile)
			if err != nil {
				return err
			}

			if err := docker.RunHooks(p.Environment.Hooks.PreStart); err != nil {
				return err
			}

			if err := docker.ComposeUp(composeFile, "."); err != nil {
				return err
			}

			connected, alreadyConnected, connErr := docker.ConnectToForgeNetwork(composeFile, ".")
			if connErr != nil {
				fmt.Println(ui.WarningStyle.Render("⚠ " + connErr.Error()))
			}
			for _, name := range connected {
				fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(name) + " " + ui.DescStyle.Render("connected to forge-network"))
			}
			for _, name := range alreadyConnected {
				fmt.Println("  " + ui.DescStyle.Render("–") + " " + ui.CmdStyle.Render(name) + " " + ui.DescStyle.Render("already on forge-network"))
			}

			if err := docker.RunHooks(p.Environment.Hooks.PostStart); err != nil {
				return err
			}

			// Display service status
			fmt.Println()
			statuses, statusErr := docker.GetServiceStatus(composeFile, ".")
			if statusErr == nil && len(statuses) > 0 {
				displayServiceStatus(statuses)
			}

			return nil
		},
	}
}

func stopCmd() *cli.Command {
	return &cli.Command{
		Name:  "stop",
		Usage: "Stop the project environment",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			p, err := config.ReadForgeRC(".")
			if err != nil {
				return fmt.Errorf("no .forgerc.json found in the current directory")
			}

			// Legacy commands format
			if p.Environment.IsLegacy() {
				fmt.Println(ui.WarningStyle.Render("⚠ .forgerc.json uses legacy \"commands\" format. Migrate to \"hooks\" + native compose."))
				return docker.RunHooks(p.Environment.Commands.Stop)
			}

			composeFile, err := docker.ResolveComposeFile(".", p.Environment.ComposeFile)
			if err != nil {
				return err
			}

			if err := docker.RunHooks(p.Environment.Hooks.PreStop); err != nil {
				return err
			}

			if err := docker.ComposeStop(composeFile, "."); err != nil {
				return err
			}

			return docker.RunHooks(p.Environment.Hooks.PostStop)
		},
	}
}

func destroyCmd() *cli.Command {
	return &cli.Command{
		Name:  "destroy",
		Usage: "Destroy the project environment",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			p, err := config.ReadForgeRC(".")
			if err != nil {
				return fmt.Errorf("no .forgerc.json found in the current directory")
			}

			// Legacy commands format
			if p.Environment.IsLegacy() {
				fmt.Println(ui.WarningStyle.Render("⚠ .forgerc.json uses legacy \"commands\" format. Migrate to \"hooks\" + native compose."))
				return docker.RunHooks(p.Environment.Commands.Destroy)
			}

			composeFile, err := docker.ResolveComposeFile(".", p.Environment.ComposeFile)
			if err != nil {
				return err
			}

			if err := docker.RunHooks(p.Environment.Hooks.PreDestroy); err != nil {
				return err
			}

			if err := docker.ComposeDown(composeFile, "."); err != nil {
				return err
			}

			return docker.RunHooks(p.Environment.Hooks.PostDestroy)
		},
	}
}

// --- Git helpers ---

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
