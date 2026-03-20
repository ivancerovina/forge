package bind

import (
	"fmt"

	"github.com/ivancerovina/forge/internal/config"
	"github.com/ivancerovina/forge/internal/system"
)

// LocalTLD is the top-level domain used for local development bindings.
// Changed from ".local" to ".test" because macOS treats .local as mDNS (Bonjour),
// causing ~5s DNS lookup delays. ".test" is IANA-reserved for testing.
const LocalTLD = ".test"

// legacyTLD is the old TLD that needs to be cleaned up on re-bind.
const legacyTLD = ".local"

type DomainBinding struct {
	Domain          string
	Container       string
	Port            int
	Path            string
	ForwardPathname bool   // true = keep path prefix when forwarding to backend
	TargetPath      string // backend target path appended to service URL
	HTTPS           bool
	Public          bool // cloudflare tunnel binding — no /etc/hosts, HTTP only
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
// If cloudflareDomain is non-empty, aliases with Cloudflare enabled will
// produce an additional public binding.
func ComputeBindings(project config.ForgeProject, cloudflareDomain string) []DomainBinding {
	var bindings []DomainBinding
	for _, entry := range project.Environment.Alias {
		container := entry.Service

		// Local binding (always generated)
		var localDomain string
		if entry.Alias == nil {
			localDomain = project.Code + LocalTLD
		} else {
			localDomain = *entry.Alias + "." + project.Code + LocalTLD
		}
		bindings = append(bindings, DomainBinding{
			Domain:          localDomain,
			Container:       container,
			Port:            entry.Port,
			Path:            entry.Path,
			ForwardPathname: entry.ForwardPathname != nil && *entry.ForwardPathname,
			TargetPath:      entry.TargetPath,
			HTTPS:           entry.HTTPS == nil || *entry.HTTPS,
		})

		// Cloudflare public binding (if enabled and domain configured)
		if entry.Cloudflare != nil && *entry.Cloudflare && cloudflareDomain != "" {
			var cfDomain string
			if entry.Alias == nil {
				cfDomain = project.Code + "." + cloudflareDomain
			} else {
				cfDomain = *entry.Alias + "." + project.Code + "." + cloudflareDomain
			}
			bindings = append(bindings, DomainBinding{
				Domain:          cfDomain,
				Container:       container,
				Port:            entry.Port,
				Path:            entry.Path,
				ForwardPathname: entry.ForwardPathname != nil && *entry.ForwardPathname,
				TargetPath:      entry.TargetPath,
				HTTPS:           false,
				Public:          true,
			})
		}
	}
	return bindings
}

// Bind adds /etc/hosts entries and writes Traefik config for the project.
func Bind(project config.ForgeProject) (*BindResult, error) {
	// Read global config for cloudflare domain
	globalCfg, _ := config.ReadConfig()

	// Validate: error if any alias has cloudflare enabled but no domain configured
	if globalCfg.CloudflareDomain == "" {
		for _, entry := range project.Environment.Alias {
			if entry.Cloudflare != nil && *entry.Cloudflare {
				return nil, fmt.Errorf("alias %q has cloudflare enabled but no cloudflare_domain is configured — run: forge tunnel set-domain <domain>", entry.Service)
			}
		}
	}

	bindings := ComputeBindings(project, globalCfg.CloudflareDomain)

	// Regenerate certificates with per-project wildcards (non-fatal)
	_ = system.RegenerateCerts()

	// Collect only local domains for /etc/hosts (skip public bindings)
	var localDomains []string
	for _, b := range bindings {
		if !b.Public {
			localDomains = append(localDomains, b.Domain)
		}
	}

	// Remove legacy .local entries for this project (migration from .local → .test)
	removeLegacyHostsEntries(project.Code)

	// Add hosts entries
	added, existing, warned, err := addHostsEntries(project.Code, localDomains)
	if err != nil {
		return nil, err
	}

	// Write traefik config (all bindings — local + public)
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
