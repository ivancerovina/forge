package docker

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServiceStatus struct {
	Name      string
	Connected bool   // has forge-network in compose YAML
	State     string // "running", "exited", etc.
	Health    string // from docker compose ps
}

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

func getContainerStates(composeFilePath, projectDir string) map[string]containerInfo {
	cmd := exec.Command("docker", "compose", "-f", composeFilePath, "ps", "-a", "--format=json")
	cmd.Dir = projectDir
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

// GetServiceStatus parses the compose file for network connectivity and
// queries docker compose for runtime state. Services are returned in
// YAML declaration order.
func GetServiceStatus(composeFilePath, projectDir string) ([]ServiceStatus, error) {
	data, err := readComposeData(composeFilePath)
	if err != nil {
		return nil, err
	}

	var compose composeFile
	var raw struct {
		Services yaml.Node `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, err
	}
	_ = yaml.Unmarshal(data, &raw)

	if len(compose.Services) == 0 {
		return nil, nil
	}

	// Preserve YAML declaration order
	type entry struct {
		name      string
		connected bool
	}
	var ordered []entry
	if raw.Services.Kind == yaml.MappingNode {
		for i := 0; i < len(raw.Services.Content)-1; i += 2 {
			name := raw.Services.Content[i].Value
			svc := compose.Services[name]
			ordered = append(ordered, entry{
				name:      name,
				connected: svc.hasNetwork("forge-network"),
			})
		}
	}

	states := getContainerStates(composeFilePath, projectDir)

	var result []ServiceStatus
	for _, e := range ordered {
		ss := ServiceStatus{
			Name:      e.name,
			Connected: e.connected,
		}
		if info, ok := states[e.name]; ok {
			ss.State = info.State
			ss.Health = info.Health
		}
		result = append(result, ss)
	}
	return result, nil
}

func readComposeData(composeFilePath string) ([]byte, error) {
	return os.ReadFile(composeFilePath)
}
