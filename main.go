package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
			tunnelCmd(),
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
			projectAliasCmd(),
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

			if err := config.WriteForgeRC(".", project); err != nil {
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
		Action: func(ctx context.Context, cmd *cli.Command) error {
			paths, err := config.ReadProjects()
			if err != nil {
				// ~/.forge/projects.json doesn't exist yet — no projects registered
				fmt.Println(ui.DescStyle.Render("No projects registered."))
				return nil
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
		},
	}
}

func projectStatusCmd() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show service connectivity status",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var composeFile string
			var err error
			var projectDir string

			loc, projErr := config.FindForgeRC(".")
			if projErr == nil {
				projectDir = loc.Dir
				composeFile, err = docker.ResolveComposeFile(loc.Dir, loc.Project.Environment.ComposeFile)
			} else {
				projectDir = "."
				composeFile, err = docker.ResolveComposeFile(".", "")
			}
			if err != nil {
				return fmt.Errorf("no compose file found")
			}

			statuses, err := docker.GetServiceStatus(composeFile, projectDir)
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
			loc, err := config.FindForgeRC(".")
			if err != nil {
				return err
			}
			project := loc.Project

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
				domainDisplay := b.Domain
				if b.Path != "" {
					domainDisplay += b.Path
				}

				if b.Public {
					// Cloudflare tunnel binding
					fmt.Printf("  %s %s → %s\n",
						ui.SuccessStyle.Render("✓"),
						ui.CmdStyle.Render("https://"+domainDisplay),
						ui.DescStyle.Render(fmt.Sprintf("%s:%d", b.Container, b.Port))+
							" "+ui.TitleStyle.Render("(cloudflare tunnel)"))
					continue
				}

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
					ui.CmdStyle.Render(scheme+"://"+domainDisplay),
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
			loc, err := config.FindForgeRC(".")
			if err != nil {
				return err
			}
			project := loc.Project

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
			loc, err := config.FindForgeRC(".")
			if err != nil {
				return err
			}
			p := loc.Project

			// Legacy commands format
			if p.Environment.IsLegacy() {
				fmt.Println(ui.WarningStyle.Render("⚠ .forgerc.json uses legacy \"commands\" format. Migrate to \"hooks\" + native compose."))
				return docker.RunHooks(p.Environment.Commands.Start, loc.Dir)
			}

			composeFile, err := docker.ResolveComposeFile(loc.Dir, p.Environment.ComposeFile)
			if err != nil {
				return err
			}

			if err := docker.RunHooks(p.Environment.Hooks.PreStart, loc.Dir); err != nil {
				return err
			}

			if err := docker.ComposeUp(composeFile, loc.Dir); err != nil {
				return err
			}

			connected, alreadyConnected, connErr := docker.ConnectToForgeNetwork(composeFile, loc.Dir)
			if connErr != nil {
				fmt.Println(ui.WarningStyle.Render("⚠ " + connErr.Error()))
			}
			for _, name := range connected {
				fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(name) + " " + ui.DescStyle.Render("connected to forge-network"))
			}
			for _, name := range alreadyConnected {
				fmt.Println("  " + ui.DescStyle.Render("–") + " " + ui.CmdStyle.Render(name) + " " + ui.DescStyle.Render("already on forge-network"))
			}

			if err := docker.RunHooks(p.Environment.Hooks.PostStart, loc.Dir); err != nil {
				return err
			}

			// Display service status
			fmt.Println()
			statuses, statusErr := docker.GetServiceStatus(composeFile, loc.Dir)
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
			loc, err := config.FindForgeRC(".")
			if err != nil {
				return err
			}
			p := loc.Project

			// Legacy commands format
			if p.Environment.IsLegacy() {
				fmt.Println(ui.WarningStyle.Render("⚠ .forgerc.json uses legacy \"commands\" format. Migrate to \"hooks\" + native compose."))
				return docker.RunHooks(p.Environment.Commands.Stop, loc.Dir)
			}

			composeFile, err := docker.ResolveComposeFile(loc.Dir, p.Environment.ComposeFile)
			if err != nil {
				return err
			}

			if err := docker.RunHooks(p.Environment.Hooks.PreStop, loc.Dir); err != nil {
				return err
			}

			if err := docker.ComposeStop(composeFile, loc.Dir); err != nil {
				return err
			}

			return docker.RunHooks(p.Environment.Hooks.PostStop, loc.Dir)
		},
	}
}

func destroyCmd() *cli.Command {
	return &cli.Command{
		Name:  "destroy",
		Usage: "Destroy the project environment",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			loc, err := config.FindForgeRC(".")
			if err != nil {
				return err
			}
			p := loc.Project

			// Legacy commands format
			if p.Environment.IsLegacy() {
				fmt.Println(ui.WarningStyle.Render("⚠ .forgerc.json uses legacy \"commands\" format. Migrate to \"hooks\" + native compose."))
				return docker.RunHooks(p.Environment.Commands.Destroy, loc.Dir)
			}

			composeFile, err := docker.ResolveComposeFile(loc.Dir, p.Environment.ComposeFile)
			if err != nil {
				return err
			}

			if err := docker.RunHooks(p.Environment.Hooks.PreDestroy, loc.Dir); err != nil {
				return err
			}

			if err := docker.ComposeDown(composeFile, loc.Dir); err != nil {
				return err
			}

			return docker.RunHooks(p.Environment.Hooks.PostDestroy, loc.Dir)
		},
	}
}

// --- Tunnel commands ---

func tunnelCmd() *cli.Command {
	return &cli.Command{
		Name:  "tunnel",
		Usage: "Manage Cloudflare tunnel integration",
		Commands: []*cli.Command{
			tunnelInitCmd(),
			tunnelStopCmd(),
			tunnelSetDomainCmd(),
			tunnelInfoCmd(),
		},
	}
}

func tunnelInitCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize Cloudflare tunnel (runs cloudflared as a Docker container)",
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			// Validate that the tunnel token env var is set
			if os.Getenv("CLOUDFLARE_TUNNEL_TOKEN") == "" {
				return fmt.Errorf("$CLOUDFLARE_TUNNEL_TOKEN is not set — export it in your shell profile")
			}

			// Enable tunnel in config
			cfg, err := config.ReadConfig()
			if err != nil {
				return fmt.Errorf("could not read config: %w", err)
			}
			cfg.CloudflareTunnel = true
			if err := config.WriteConfig(cfg); err != nil {
				return fmt.Errorf("could not write config: %w", err)
			}

			// Write cf-config.yml and regenerate compose file
			forgeDir, err := config.ForgeDir()
			if err != nil {
				return fmt.Errorf("could not determine forge directory: %w", err)
			}
			if err := system.WriteCFConfig(forgeDir); err != nil {
				return err
			}
			if err := system.WriteComposeFile(forgeDir); err != nil {
				return err
			}

			// Start the stack
			if err := system.StartTraefik(forgeDir); err != nil {
				return err
			}

			fmt.Println(ui.TitleStyle.Render("Cloudflare tunnel initialized!"))
			fmt.Println()
			fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " +
				ui.CmdStyle.Render("cloudflared container") + " " +
				ui.SuccessStyle.Render("started"))
			fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " +
				ui.CmdStyle.Render("~/.forge/cf-config.yml") + " " +
				ui.SuccessStyle.Render("written"))
			return nil
		}),
	}
}

func tunnelStopCmd() *cli.Command {
	return &cli.Command{
		Name:  "stop",
		Usage: "Stop and remove the Cloudflare tunnel container",
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.ReadConfig()
			if err != nil {
				return fmt.Errorf("could not read config: %w", err)
			}
			if !cfg.CloudflareTunnel {
				return fmt.Errorf("tunnel is not enabled — nothing to stop")
			}

			cfg.CloudflareTunnel = false
			if err := config.WriteConfig(cfg); err != nil {
				return fmt.Errorf("could not write config: %w", err)
			}

			// Regenerate compose (cloudflared removed) and restart
			forgeDir, err := config.ForgeDir()
			if err != nil {
				return fmt.Errorf("could not determine forge directory: %w", err)
			}
			if err := system.WriteComposeFile(forgeDir); err != nil {
				return err
			}
			if err := system.StartTraefik(forgeDir); err != nil {
				return err
			}

			fmt.Println(ui.TitleStyle.Render("Cloudflare tunnel stopped!"))
			fmt.Println()
			fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " +
				ui.CmdStyle.Render("cloudflared container") + " " +
				ui.DescStyle.Render("removed"))
			return nil
		}),
	}
}

func tunnelSetDomainCmd() *cli.Command {
	return &cli.Command{
		Name:      "set-domain",
		Usage:     "Set the Cloudflare tunnel base domain",
		ArgsUsage: "<domain>",
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			domain := cmd.Args().First()
			if domain == "" {
				return fmt.Errorf("domain argument is required (e.g. forge tunnel set-domain dev.example.com)")
			}

			if err := config.ValidateDomain(domain); err != nil {
				return err
			}

			cfg, err := config.ReadConfig()
			if err != nil {
				return fmt.Errorf("could not read config: %w", err)
			}

			cfg.CloudflareDomain = domain

			if err := config.WriteConfig(cfg); err != nil {
				return fmt.Errorf("could not write config: %w", err)
			}

			fmt.Printf("  %s Cloudflare domain set to %s\n",
				ui.SuccessStyle.Render("✓"),
				ui.CmdStyle.Render(domain))
			return nil
		}),
	}
}

func tunnelInfoCmd() *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Show current tunnel configuration",
		Action: withInit(func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := config.ReadConfig()
			if err != nil {
				return fmt.Errorf("could not read config: %w", err)
			}

			fmt.Println(ui.HeadingStyle.Render("Tunnel configuration:"))
			fmt.Println()
			if cfg.CloudflareDomain != "" {
				fmt.Println("  " + ui.DescStyle.Render("Cloudflare domain:") + " " + ui.CmdStyle.Render(cfg.CloudflareDomain))
			} else {
				fmt.Println("  " + ui.DescStyle.Render("Cloudflare domain:") + " " + ui.DescStyle.Render("not configured"))
			}

			if cfg.CloudflareTunnel {
				fmt.Println("  " + ui.DescStyle.Render("Tunnel:") + "           " + ui.SuccessStyle.Render("enabled"))
				state := getContainerState("forge-cloudflared")
				fmt.Println("  " + ui.DescStyle.Render("Container:") + "        " +
					stateIndicator(state) + " " + stateLabel(state))
			} else {
				fmt.Println("  " + ui.DescStyle.Render("Tunnel:") + "           " + ui.DescStyle.Render("disabled"))
			}
			return nil
		}),
	}
}

func getContainerState(containerName string) string {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// --- Alias commands ---

func projectAliasCmd() *cli.Command {
	return &cli.Command{
		Name:  "alias",
		Usage: "Manage project service aliases",
		Commands: []*cli.Command{
			projectAliasAddCmd(),
			projectAliasRemoveCmd(),
			projectAliasInfoCmd(),
		},
	}
}

func projectAliasAddCmd() *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Add a service alias",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", Aliases: []string{"P"}, Usage: "Service port"},
			&cli.StringFlag{Name: "alias", Aliases: []string{"a"}, Usage: "Subdomain (omit for index domain)"},
			&cli.StringFlag{Name: "path", Usage: "Path prefix (e.g. /api)"},
			&cli.BoolFlag{Name: "http", Usage: "HTTP only (default is HTTPS)"},
			&cli.BoolFlag{Name: "cloudflare", Usage: "Also bind via Cloudflare tunnel"},
			&cli.BoolFlag{Name: "force", Usage: "Overwrite existing alias"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			loc, err := config.FindForgeRC(".")
			if err != nil {
				return err
			}
			project := loc.Project

			serviceName := cmd.Args().First()
			portSet := cmd.IsSet("port")
			interactive := serviceName == "" && !portSet

			var port int
			var alias string
			var path string
			var https bool
			var cloudflare bool

			if interactive {
				// Interactive mode
				var portStr string
				https = true

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Service name").
							Value(&serviceName).
							Validate(func(s string) error { return config.ValidateServiceName(s) }),
						huh.NewInput().
							Title("Port").
							Value(&portStr).
							Validate(func(s string) error {
								p, err := strconv.Atoi(s)
								if err != nil {
									return fmt.Errorf("port must be a number")
								}
								return config.ValidatePort(p)
							}),
						huh.NewInput().
							Title("Alias subdomain").
							Description("Leave empty for index domain (<code>.local)").
							Value(&alias).
							Validate(func(s string) error { return config.ValidateAliasSubdomain(s) }),
						huh.NewInput().
							Title("Path prefix").
							Description("Leave empty for no path prefix (e.g. /api)").
							Value(&path).
							Validate(func(s string) error { return config.ValidatePath(s) }),
						huh.NewConfirm().
							Title("Enable HTTPS?").
							Affirmative("Yes").
							Negative("No").
							Value(&https),
						huh.NewConfirm().
							Title("Bind via Cloudflare tunnel?").
							Affirmative("Yes").
							Negative("No").
							Value(&cloudflare),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}

				port, _ = strconv.Atoi(portStr)

				// Check for existing entry
				if project.Environment.Alias != nil {
					if _, exists := project.Environment.Alias[serviceName]; exists {
						var overwrite bool
						confirmForm := huh.NewForm(
							huh.NewGroup(
								huh.NewConfirm().
									Title(fmt.Sprintf("Alias for %q already exists. Overwrite?", serviceName)).
									Affirmative("Yes").
									Negative("No").
									Value(&overwrite),
							),
						)
						if err := confirmForm.Run(); err != nil {
							return err
						}
						if !overwrite {
							return nil
						}
					}
				}
			} else {
				// Non-interactive mode
				if serviceName == "" {
					return fmt.Errorf("service name is required as a positional argument")
				}
				if !portSet {
					return fmt.Errorf("--port is required")
				}

				port = int(cmd.Int("port"))
				alias = cmd.String("alias")
				path = cmd.String("path")
				https = !cmd.Bool("http")
				cloudflare = cmd.Bool("cloudflare")

				if err := config.ValidateServiceName(serviceName); err != nil {
					return err
				}
				if err := config.ValidatePort(port); err != nil {
					return err
				}
				if err := config.ValidateAliasSubdomain(alias); err != nil {
					return err
				}
				if err := config.ValidatePath(path); err != nil {
					return err
				}

				// Check for existing entry
				if project.Environment.Alias != nil {
					if _, exists := project.Environment.Alias[serviceName]; exists && !cmd.Bool("force") {
						return fmt.Errorf("alias for %q already exists (use --force to overwrite)", serviceName)
					}
				}
			}

			// Build entry
			entry := config.AliasEntry{Port: port, Path: path}
			if alias == "" {
				entry.Alias = nil
			} else {
				entry.Alias = stringPtr(alias)
			}
			if !https {
				entry.HTTPS = boolPtr(false)
			}
			if cloudflare {
				entry.Cloudflare = boolPtr(true)
			}

			// Guard nil map
			if project.Environment.Alias == nil {
				project.Environment.Alias = make(map[string]config.AliasEntry)
			}
			project.Environment.Alias[serviceName] = entry

			if err := config.WriteForgeRC(loc.Dir, project); err != nil {
				return err
			}

			// Compute domain for display
			var domain string
			if alias == "" {
				domain = project.Code + ".local"
			} else {
				domain = alias + "." + project.Code + ".local"
			}
			if path != "" {
				domain += path
			}
			scheme := "HTTPS"
			if !https {
				scheme = "HTTP"
			}

			cfLabel := ""
			if cloudflare {
				cfLabel = "  " + ui.TitleStyle.Render("CF")
			}

			fmt.Printf("  %s %s  %d → %s  %s%s\n",
				ui.SuccessStyle.Render("✓"),
				ui.CmdStyle.Render(serviceName),
				port,
				ui.DescStyle.Render(domain),
				ui.TitleStyle.Render(scheme),
				cfLabel)

			return nil
		},
	}
}

func projectAliasRemoveCmd() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Aliases: []string{"rm"},
		Usage:   "Remove a service alias",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			loc, err := config.FindForgeRC(".")
			if err != nil {
				return err
			}
			project := loc.Project

			serviceName := cmd.Args().First()
			interactive := serviceName == ""

			if interactive {
				if len(project.Environment.Alias) == 0 {
					fmt.Println(ui.DescStyle.Render("No aliases defined."))
					return nil
				}

				// Build sorted list of service names
				names := make([]string, 0, len(project.Environment.Alias))
				for k := range project.Environment.Alias {
					names = append(names, k)
				}
				sort.Strings(names)

				options := make([]huh.Option[string], len(names))
				for i, n := range names {
					options[i] = huh.NewOption(n, n)
				}

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("Select alias to remove").
							Options(options...).
							Value(&serviceName),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}

				var confirm bool
				confirmForm := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title(fmt.Sprintf("Remove alias for %q?", serviceName)).
							Affirmative("Yes").
							Negative("No").
							Value(&confirm),
					),
				)
				if err := confirmForm.Run(); err != nil {
					return err
				}
				if !confirm {
					return nil
				}
			} else {
				if project.Environment.Alias == nil {
					return fmt.Errorf("alias %q not found", serviceName)
				}
				if _, exists := project.Environment.Alias[serviceName]; !exists {
					return fmt.Errorf("alias %q not found", serviceName)
				}
			}

			delete(project.Environment.Alias, serviceName)

			if err := config.WriteForgeRC(loc.Dir, project); err != nil {
				return err
			}

			fmt.Printf("  %s %s  %s\n",
				ui.SuccessStyle.Render("✓"),
				ui.CmdStyle.Render(serviceName),
				ui.DescStyle.Render("removed"))

			return nil
		},
	}
}

func projectAliasInfoCmd() *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Show alias details",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			loc, err := config.FindForgeRC(".")
			if err != nil {
				return err
			}
			project := loc.Project

			serviceName := cmd.Args().First()

			if serviceName != "" {
				// Show single alias
				if project.Environment.Alias == nil {
					return fmt.Errorf("alias %q not found", serviceName)
				}
				entry, exists := project.Environment.Alias[serviceName]
				if !exists {
					return fmt.Errorf("alias %q not found", serviceName)
				}

				var domain string
				if entry.Alias == nil {
					domain = project.Code + ".local"
				} else {
					domain = *entry.Alias + "." + project.Code + ".local"
				}
				https := entry.HTTPS == nil || *entry.HTTPS
				httpsLabel := "yes"
				if !https {
					httpsLabel = "no"
				}

				cfLabel := "no"
				if entry.Cloudflare != nil && *entry.Cloudflare {
					cfLabel = "yes"
				}

				fmt.Println("  " + ui.CmdStyle.Render(serviceName))
				fmt.Println("    " + ui.DescStyle.Render("Port:") + "       " + ui.CmdStyle.Render(strconv.Itoa(entry.Port)))
				fmt.Println("    " + ui.DescStyle.Render("Domain:") + "     " + ui.CmdStyle.Render(domain))
				if entry.Path != "" {
					fmt.Println("    " + ui.DescStyle.Render("Path:") + "       " + ui.CmdStyle.Render(entry.Path))
				}
				fmt.Println("    " + ui.DescStyle.Render("HTTPS:") + "      " + ui.CmdStyle.Render(httpsLabel))
				if entry.Cloudflare != nil && *entry.Cloudflare {
					fmt.Println("    " + ui.DescStyle.Render("Cloudflare:") + " " + ui.CmdStyle.Render(cfLabel))
				}
				return nil
			}

			// Show all aliases
			if len(project.Environment.Alias) == 0 {
				fmt.Println(ui.DescStyle.Render("No aliases defined."))
				return nil
			}

			globalCfg, _ := config.ReadConfig()
			bindings := bind.ComputeBindings(project, globalCfg.CloudflareDomain)

			// Compute column widths
			maxName := 0
			maxPort := 0
			for _, b := range bindings {
				if len(b.Container) > maxName {
					maxName = len(b.Container)
				}
				portStr := strconv.Itoa(b.Port)
				if len(portStr) > maxPort {
					maxPort = len(portStr)
				}
			}

			fmt.Println(ui.HeadingStyle.Render("Aliases:"))
			fmt.Println()
			for _, b := range bindings {
				portStr := strconv.Itoa(b.Port)
				namePad := maxName - len(b.Container) + 2
				portPad := maxPort - len(portStr) + 2

				domainDisplay := b.Domain
				if b.Path != "" {
					domainDisplay += b.Path
				}

				var schemeLabel string
				if b.Public {
					schemeLabel = ui.TitleStyle.Render("CF")
				} else if b.HTTPS {
					schemeLabel = ui.TitleStyle.Render("HTTPS")
				} else {
					schemeLabel = ui.WarningStyle.Render("HTTP")
				}

				fmt.Printf("  %s%*s%s%*s→  %s  %s\n",
					ui.CmdStyle.Render(b.Container),
					namePad, "",
					ui.CmdStyle.Render(portStr),
					portPad, "",
					ui.DescStyle.Render(domainDisplay),
					schemeLabel)
			}

			return nil
		},
	}
}

// --- Pointer helpers ---

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }

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
		// If "add" failed because remote already exists, try set-url instead
		if setErr := runGit("remote", "set-url", "origin", url); setErr != nil {
			return fmt.Errorf("failed to configure git remote: %w", setErr)
		}
	}
	return nil
}
