package bind

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// writeHostsFile writes content to /etc/hosts via sudo tee, prompting the user
// for their password if needed.
func writeHostsFile(content []byte) error {
	cmd := exec.Command("sudo", "tee", "/etc/hosts")
	cmd.Stdin = bytes.NewReader(content)
	cmd.Stdout = nil       // suppress tee's stdout echo
	cmd.Stderr = os.Stderr // show sudo password prompt + errors
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not write /etc/hosts (sudo failed): %w", err)
	}
	return nil
}

// addHostsEntries adds 127.0.0.1 entries to /etc/hosts for the given domains,
// tagged with a marker comment for later removal. Returns which domains were
// newly added, already managed by this project, or warned (present from another
// source but added with a forge marker anyway).
func addHostsEntries(projectCode string, domains []string) (added, existing, warned []string, err error) {
	data, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not read /etc/hosts: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	marker := "# forge:" + projectCode

	// Build set of domains already managed by this project's marker
	forgeDomains := make(map[string]bool)
	// Build set of all domains present in /etc/hosts (any source)
	allDomains := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		isForgeManaged := strings.Contains(line, marker)
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "#") {
				break
			}
			allDomains[f] = true
			if isForgeManaged {
				forgeDomains[f] = true
			}
		}
	}

	var newLines []string
	for _, domain := range domains {
		if forgeDomains[domain] {
			// Already managed by this project — skip
			existing = append(existing, domain)
			continue
		}
		if allDomains[domain] {
			// Present from another source — add forge entry anyway but warn
			warned = append(warned, domain)
		}
		newLines = append(newLines, "127.0.0.1 "+domain+" "+marker)
		added = append(added, domain)
	}

	if len(newLines) == 0 {
		return added, existing, warned, nil
	}

	// Ensure file ends with newline before appending
	content := string(data)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += strings.Join(newLines, "\n") + "\n"

	if err := writeHostsFile([]byte(content)); err != nil {
		return nil, nil, nil, err
	}

	return added, existing, warned, nil
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

	if err := writeHostsFile([]byte(content)); err != nil {
		return nil, err
	}

	return removed, nil
}
