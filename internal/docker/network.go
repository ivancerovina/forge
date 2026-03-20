package docker

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConnectToForgeNetwork connects all services in the compose file to forge-network.
// Returns lists of newly connected and already-connected service names.
func ConnectToForgeNetwork(composeFilePath, projectDir string) (connected, alreadyConnected []string, err error) {
	data, err := readComposeData(composeFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read compose file: %w", err)
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, nil, fmt.Errorf("could not parse compose file: %w", err)
	}

	// Get running container IDs for each service
	for name := range compose.Services {
		containerID, err := getContainerID(composeFilePath, projectDir, name)
		if err != nil || containerID == "" {
			continue
		}

		ok, err := IsConnectedToNetwork(containerID, "forge-network")
		if err != nil {
			continue
		}

		if ok {
			alreadyConnected = append(alreadyConnected, name)
			continue
		}

		cmd := exec.Command("docker", "network", "connect", "--alias", name, "forge-network", containerID)
		if err := cmd.Run(); err != nil {
			return connected, alreadyConnected, fmt.Errorf("failed to connect %s to forge-network: %w", name, err)
		}
		connected = append(connected, name)
	}

	return connected, alreadyConnected, nil
}

// IsConnectedToNetwork checks if a container is connected to the given network.
func IsConnectedToNetwork(containerID, network string) (bool, error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{json .NetworkSettings.Networks}}", containerID)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("could not inspect container %s: %w", containerID, err)
	}

	var networks map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &networks); err != nil {
		return false, err
	}

	_, ok := networks[network]
	return ok, nil
}

func getContainerID(composeFilePath, projectDir, service string) (string, error) {
	cmd := exec.Command("docker", "compose", "-f", composeFilePath, "ps", "-q", service)
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// CheckAliasKeys checks whether .forgerc.json alias keys match compose service
// names instead of container names. Returns warnings for any alias key that
// matches a service name that has a different container_name set — meaning
// Traefik won't be able to resolve it on forge-network.
func CheckAliasKeys(composeFilePath string, aliasKeys []string) []string {
	data, err := readComposeData(composeFilePath)
	if err != nil {
		return nil
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil
	}

	// Build lookup maps
	serviceNames := make(map[string]string) // service name → container_name (if set)
	containerNames := make(map[string]bool)  // all known container names
	for name, svc := range compose.Services {
		serviceNames[name] = svc.ContainerName
		if svc.ContainerName != "" {
			containerNames[svc.ContainerName] = true
		}
	}

	var warnings []string
	for _, key := range aliasKeys {
		// If the key matches a service name that has a different container_name
		if containerName, isService := serviceNames[key]; isService && containerName != "" && containerName != key {
			warnings = append(warnings, fmt.Sprintf(
				"alias %q matches compose service name, but container_name is %q — use the container name instead",
				key, containerName,
			))
			continue
		}

		// If the key doesn't match any service name or container name, it won't resolve
		_, isService := serviceNames[key]
		if !isService && !containerNames[key] {
			warnings = append(warnings, fmt.Sprintf(
				"alias %q does not match any compose service or container name — Traefik won't be able to route to it",
				key,
			))
		}
	}

	return warnings
}
