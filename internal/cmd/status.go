package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ivancerovina/forge/internal/ui"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Networks yaml.Node `yaml:"networks"`
}

func (s *composeService) hasNetwork(name string) bool {
	if s.Networks.Kind == 0 {
		return false
	}
	// Sequence form: networks: [forge-network, internal]
	if s.Networks.Kind == yaml.SequenceNode {
		for _, n := range s.Networks.Content {
			if n.Value == name {
				return true
			}
		}
	}
	// Mapping form: networks: { forge-network: ... }
	if s.Networks.Kind == yaml.MappingNode {
		for i := 0; i < len(s.Networks.Content)-1; i += 2 {
			if s.Networks.Content[i].Value == name {
				return true
			}
		}
	}
	return false
}

type containerInfo struct {
	Service string `json:"Service"`
	State   string `json:"State"`
	Health  string `json:"Health"`
}

func getContainerStates() map[string]containerInfo {
	cmd := exec.Command("docker", "compose", "ps", "-a", "--format=json")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	states := make(map[string]containerInfo)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var c containerInfo
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		states[c.Service] = c
	}
	return states
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

func StatusHelp() {
	fmt.Println(ui.TitleStyle.Render("forge project status") + ui.DescStyle.Render(" - show service status"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge project status") + "  " + ui.DescStyle.Render("Show status of compose services"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))
}

func Status(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			StatusHelp()
			os.Exit(0)
		}
	}

	data, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		data, err = os.ReadFile("docker-compose.yaml")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No docker-compose.yml found in the current directory."))
		os.Exit(1)
	}

	// Parse into both structured data and raw nodes (for declaration order)
	var compose composeFile
	var raw struct {
		Services yaml.Node `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("Failed to parse compose file: "+err.Error()))
		os.Exit(1)
	}
	_ = yaml.Unmarshal(data, &raw)

	if len(compose.Services) == 0 {
		fmt.Fprintln(os.Stderr, ui.ErrStyle.Render("No services found in compose file."))
		os.Exit(1)
	}

	// Preserve declaration order from YAML
	type serviceEntry struct {
		name      string
		connected bool
	}

	var ordered []serviceEntry
	if raw.Services.Kind == yaml.MappingNode {
		for i := 0; i < len(raw.Services.Content)-1; i += 2 {
			name := raw.Services.Content[i].Value
			svc := compose.Services[name]
			ordered = append(ordered, serviceEntry{
				name:      name,
				connected: svc.hasNetwork("forge-network"),
			})
		}
	}

	// Get runtime state from docker compose
	states := getContainerStates()

	// Find longest service name for alignment
	maxLen := 0
	for _, e := range ordered {
		if len(e.name) > maxLen {
			maxLen = len(e.name)
		}
	}

	fmt.Println(ui.HeadingStyle.Render("Services:"))
	fmt.Println()
	for _, e := range ordered {
		pad := maxLen - len(e.name) + 2

		// Connection icon
		var connIcon string
		if e.connected {
			connIcon = ui.TitleStyle.Render("✓")
		} else {
			connIcon = ui.DescStyle.Render("–")
		}

		// State from docker
		info, hasContainer := states[e.name]
		var statusText string
		var indicator string
		if hasContainer {
			indicator = stateIndicator(info.State)
			statusText = stateLabel(info.State)
			if info.Health != "" {
				statusText += ui.DescStyle.Render(" ("+info.Health+")")
			}
		} else {
			indicator = stateIndicator("")
			statusText = ui.DescStyle.Render("not created")
		}

		fmt.Printf("  %s %s %s%*s%s\n",
			indicator,
			connIcon,
			ui.CmdStyle.Render(e.name),
			pad, "",
			statusText)
	}
}
