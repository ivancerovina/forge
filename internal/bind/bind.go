package bind

import (
	"sort"

	"github.com/ivancerovina/forge/internal/config"
	"github.com/ivancerovina/forge/internal/system"
)

type DomainBinding struct {
	Domain    string
	Container string
	Port      int
	HTTPS     bool
}

type BindResult struct {
	Bindings        []DomainBinding
	AddedDomains    []string
	ExistingDomains []string
	WarnedDomains   []string
	TraefikPath     string
	HasCerts        bool
}

type UnbindResult struct {
	RemovedDomains []string
	TraefikPath    string
}

// ComputeBindings computes domain bindings from project alias configuration.
// Keys are sorted for deterministic output.
func ComputeBindings(project config.ForgeProject) []DomainBinding {
	// Sort alias map keys for deterministic order
	keys := make([]string, 0, len(project.Environment.Alias))
	for k := range project.Environment.Alias {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var bindings []DomainBinding
	for _, container := range keys {
		entry := project.Environment.Alias[container]
		var domain string
		if entry.Alias == nil {
			domain = project.Code + ".local"
		} else {
			domain = *entry.Alias + "." + project.Code + ".local"
		}
		bindings = append(bindings, DomainBinding{
			Domain:    domain,
			Container: container,
			Port:      entry.Port,
			HTTPS:     entry.HTTPS == nil || *entry.HTTPS,
		})
	}
	return bindings
}

// Bind adds /etc/hosts entries and writes Traefik config for the project.
func Bind(project config.ForgeProject) (*BindResult, error) {
	bindings := ComputeBindings(project)

	// Regenerate certificates with per-project wildcards (non-fatal)
	_ = system.RegenerateCerts()

	// Collect domains for /etc/hosts
	domains := make([]string, len(bindings))
	for i, b := range bindings {
		domains[i] = b.Domain
	}

	// Add hosts entries
	added, existing, warned, err := addHostsEntries(project.Code, domains)
	if err != nil {
		return nil, err
	}

	// Write traefik config
	if err := writeTraefikConfig(project, bindings); err != nil {
		return nil, err
	}

	path, _ := traefikConfigPath(project.Code)
	hasCerts := system.CertsAvailable()

	return &BindResult{
		Bindings:        bindings,
		AddedDomains:    added,
		ExistingDomains: existing,
		WarnedDomains:   warned,
		TraefikPath:     path,
		HasCerts:        hasCerts,
	}, nil
}

// Unbind removes /etc/hosts entries and Traefik config for the project.
func Unbind(project config.ForgeProject) (*UnbindResult, error) {
	removed, err := removeHostsEntries(project.Code)
	if err != nil {
		return nil, err
	}

	if err := removeTraefikConfig(project.Code); err != nil {
		return nil, err
	}

	path, _ := traefikConfigPath(project.Code)

	return &UnbindResult{
		RemovedDomains: removed,
		TraefikPath:    path,
	}, nil
}
