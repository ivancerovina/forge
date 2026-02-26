package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ivancerovina/forge/internal/ui"
)

type domainBinding struct {
	Domain    string
	Container string
	Port      int
	HTTPS     bool
}

func requireRoot(command string) {
	if os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("forge project "+command+" requires root privileges."))
		fmt.Fprintln(os.Stderr, "  Run with "+ui.CmdStyle.Render("sudo forge project "+command))
		os.Exit(1)
	}
}

func computeBindings(project ForgeProject) []domainBinding {
	var bindings []domainBinding
	for container, entry := range project.Environment.Alias {
		var domain string
		if entry.Alias == nil {
			domain = project.Code + ".local"
		} else {
			domain = *entry.Alias + "." + project.Code + ".local"
		}
		bindings = append(bindings, domainBinding{
			Domain:    domain,
			Container: container,
			Port:      entry.Port,
			HTTPS:     entry.HTTPS == nil || *entry.HTTPS,
		})
	}
	return bindings
}

func traefikConfigPath(projectCode string) (string, error) {
	home, err := UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".forge", "traefik", projectCode+".yml"), nil
}

// addHostsEntries adds 127.0.0.1 entries to /etc/hosts for the given domains,
// tagged with a marker comment for later removal. Returns which domains were
// newly added vs already present.
func addHostsEntries(projectCode string, domains []string) (added, existing []string, err error) {
	data, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return nil, nil, fmt.Errorf("could not read /etc/hosts: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// Build set of domains already in /etc/hosts (any line, not just ours)
	presentDomains := make(map[string]bool)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "#") {
				break
			}
			presentDomains[f] = true
		}
	}

	marker := "# forge:" + projectCode
	var newLines []string
	for _, domain := range domains {
		if presentDomains[domain] {
			existing = append(existing, domain)
			continue
		}
		newLines = append(newLines, "127.0.0.1 "+domain+" "+marker)
		added = append(added, domain)
	}

	if len(newLines) == 0 {
		return added, existing, nil
	}

	// Ensure file ends with newline before appending
	content := string(data)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += strings.Join(newLines, "\n") + "\n"

	if err := os.WriteFile("/etc/hosts", []byte(content), 0o644); err != nil {
		return nil, nil, fmt.Errorf("could not write /etc/hosts: %w", err)
	}

	return added, existing, nil
}

// removeHostsEntries removes all /etc/hosts lines tagged with the project's marker.
func removeHostsEntries(projectCode string) (removed []string, err error) {
	f, err := os.Open("/etc/hosts")
	if err != nil {
		return nil, fmt.Errorf("could not read /etc/hosts: %w", err)
	}
	defer f.Close()

	marker := "# forge:" + projectCode
	var keep []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, marker) {
			// Extract the domain from the line for reporting
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				removed = append(removed, fields[1])
			}
			continue
		}
		keep = append(keep, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("could not read /etc/hosts: %w", err)
	}

	if len(removed) == 0 {
		return nil, nil
	}

	content := strings.Join(keep, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := os.WriteFile("/etc/hosts", []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("could not write /etc/hosts: %w", err)
	}

	return removed, nil
}

// traefikDynamicConfig represents the Traefik file provider YAML structure.
type traefikDynamicConfig struct {
	HTTP struct {
		Routers  map[string]traefikRouter  `yaml:"routers"`
		Services map[string]traefikService `yaml:"services"`
	} `yaml:"http"`
}

type traefikRouterTLS struct{} // empty struct — produces `tls: {}` in YAML

type traefikRouter struct {
	Rule        string            `yaml:"rule"`
	Service     string            `yaml:"service"`
	EntryPoints []string          `yaml:"entryPoints"`
	TLS         *traefikRouterTLS `yaml:"tls,omitempty"`
}

type traefikService struct {
	LoadBalancer traefikLB `yaml:"loadBalancer"`
}

type traefikLB struct {
	Servers []traefikServer `yaml:"servers"`
}

type traefikServer struct {
	URL string `yaml:"url"`
}

func certsAvailable() bool {
	home, err := UserHomeDir()
	if err != nil {
		return false
	}
	certPath := filepath.Join(home, ".forge", "certs", "local.pem")
	if _, err := os.Stat(certPath); err != nil {
		return false
	}
	return true
}

func writeTraefikConfig(project ForgeProject, bindings []domainBinding) error {
	var cfg traefikDynamicConfig
	cfg.HTTP.Routers = make(map[string]traefikRouter)
	cfg.HTTP.Services = make(map[string]traefikService)

	hasCerts := certsAvailable()

	for _, b := range bindings {
		key := project.Code + "-" + b.Container
		router := traefikRouter{
			Rule:    fmt.Sprintf("Host(`%s`)", b.Domain),
			Service: key,
		}

		if b.HTTPS && hasCerts {
			router.EntryPoints = []string{"websecure"}
			router.TLS = &traefikRouterTLS{}
		} else {
			router.EntryPoints = []string{"web"}
		}

		cfg.HTTP.Routers[key] = router
		cfg.HTTP.Services[key] = traefikService{
			LoadBalancer: traefikLB{
				Servers: []traefikServer{
					{URL: fmt.Sprintf("http://%s:%d", b.Container, b.Port)},
				},
			},
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal traefik config: %w", err)
	}

	path, err := traefikConfigPath(project.Code)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("could not write traefik config: %w", err)
	}

	return nil
}

func removeTraefikConfig(projectCode string) error {
	path, err := traefikConfigPath(projectCode)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove traefik config: %w", err)
	}
	return nil
}

func BindHelp() {
	fmt.Println(ui.TitleStyle.Render("forge project bind") + ui.DescStyle.Render(" - bind project domains to local routing"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("sudo forge project bind") + "  " + ui.DescStyle.Render("Bind domains from .forgerc.json aliases"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Adds /etc/hosts entries and configures Traefik routing"))
	fmt.Println(ui.DescStyle.Render("for each service alias defined in ") + ui.CmdStyle.Render("environment.alias") + ui.DescStyle.Render("."))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Requires root privileges (sudo)."))
}

func Bind(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			BindHelp()
			os.Exit(0)
		}
	}

	requireRoot("bind")

	project, err := readForgeRCAt(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No .forgerc.json found in the current directory."))
		os.Exit(1)
	}

	if len(project.Environment.Alias) == 0 {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No aliases defined in .forgerc.json."))
		fmt.Fprintln(os.Stderr, ui.DescStyle.Render("Add entries to ")+ui.CmdStyle.Render("environment.alias")+ui.DescStyle.Render(" first."))
		os.Exit(1)
	}

	bindings := computeBindings(project)

	// Regenerate certificates with per-project wildcards (non-fatal)
	if err := RegenerateCerts(); err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.WarningStyle.Render("!")+" "+ui.DescStyle.Render("certificate regeneration failed: "+err.Error()))
	}

	// Collect domains for /etc/hosts
	domains := make([]string, len(bindings))
	for i, b := range bindings {
		domains[i] = b.Domain
	}

	// Add hosts entries
	_, existing, err := addHostsEntries(project.Code, domains)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	// Write traefik config
	if err := writeTraefikConfig(project, bindings); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	hasCerts := certsAvailable()

	// Print summary
	fmt.Println(ui.TitleStyle.Render("Project domains bound!"))
	fmt.Println()
	for _, b := range bindings {
		status := ui.SuccessStyle.Render("added")
		for _, e := range existing {
			if e == b.Domain {
				status = ui.DescStyle.Render("already in /etc/hosts")
				break
			}
		}
		scheme := "http"
		if b.HTTPS && hasCerts {
			scheme = "https"
		}
		fmt.Printf("  %s %s → %s\n",
			ui.SuccessStyle.Render("✓"),
			ui.CmdStyle.Render(scheme+"://"+b.Domain),
			ui.DescStyle.Render(fmt.Sprintf("%s:%d", b.Container, b.Port))+
				" "+status)
	}

	path, _ := traefikConfigPath(project.Code)
	fmt.Println()
	fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(path) + " " + ui.SuccessStyle.Render("written"))
}

func UnbindHelp() {
	fmt.Println(ui.TitleStyle.Render("forge project unbind") + ui.DescStyle.Render(" - remove project domain bindings"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("sudo forge project unbind") + "  " + ui.DescStyle.Render("Remove domains for the current project"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Removes /etc/hosts entries and Traefik config for this project."))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Requires root privileges (sudo)."))
}

func Unbind(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			UnbindHelp()
			os.Exit(0)
		}
	}

	requireRoot("unbind")

	project, err := readForgeRCAt(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No .forgerc.json found in the current directory."))
		os.Exit(1)
	}

	// Remove hosts entries
	removed, err := removeHostsEntries(project.Code)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	// Remove traefik config
	if err := removeTraefikConfig(project.Code); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render(err.Error()))
		os.Exit(1)
	}

	// Print summary
	if len(removed) == 0 {
		fmt.Println(ui.DescStyle.Render("No hosts entries found for project ") + ui.CmdStyle.Render(project.Code) + ui.DescStyle.Render("."))
	} else {
		fmt.Println(ui.TitleStyle.Render("Project domains unbound!"))
		fmt.Println()
		for _, domain := range removed {
			fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(domain) + " " + ui.DescStyle.Render("removed from /etc/hosts"))
		}
	}

	path, _ := traefikConfigPath(project.Code)
	fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render(path) + " " + ui.DescStyle.Render("removed"))
}
