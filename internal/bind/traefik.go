package bind

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/ivancerovina/forge/internal/config"
	"github.com/ivancerovina/forge/internal/system"
)

type traefikDynamicConfig struct {
	HTTP struct {
		Routers     map[string]traefikRouter     `yaml:"routers"`
		Services    map[string]traefikService     `yaml:"services"`
		Middlewares map[string]traefikMiddleware   `yaml:"middlewares,omitempty"`
	} `yaml:"http"`
}

type traefikMiddleware struct {
	RedirectScheme *traefikRedirectScheme `yaml:"redirectScheme,omitempty"`
	StripPrefix    *traefikStripPrefix    `yaml:"stripPrefix,omitempty"`
}

type traefikStripPrefix struct {
	Prefixes []string `yaml:"prefixes"`
}

type traefikRedirectScheme struct {
	Scheme    string `yaml:"scheme"`
	Permanent bool   `yaml:"permanent"`
}

type traefikRouterTLS struct{} // empty struct — produces `tls: {}` in YAML

type traefikRouter struct {
	Rule        string            `yaml:"rule"`
	Service     string            `yaml:"service"`
	EntryPoints []string          `yaml:"entryPoints"`
	Middlewares []string          `yaml:"middlewares,omitempty"`
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

func traefikConfigPath(projectCode string) (string, error) {
	forgeDir, err := config.ForgeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(forgeDir, "traefik", projectCode+".yml"), nil
}

func writeTraefikConfig(project config.ForgeProject, bindings []DomainBinding) error {
	var cfg traefikDynamicConfig
	cfg.HTTP.Routers = make(map[string]traefikRouter)
	cfg.HTTP.Services = make(map[string]traefikService)

	hasCerts := system.CertsAvailable()

	for _, b := range bindings {
		serviceKey := project.Code + "-" + b.Container
		routerKey := serviceKey
		if b.Public {
			routerKey = serviceKey + "-cf"
		}

		rule := fmt.Sprintf("Host(`%s`)", b.Domain)
		if b.Path != "" {
			rule = fmt.Sprintf("Host(`%s`) && PathPrefix(`%s`)", b.Domain, b.Path)
		}

		router := traefikRouter{
			Rule:    rule,
			Service: serviceKey,
		}

		// Add StripPrefix middleware for path-based routes (unless forwarding pathname)
		if b.Path != "" && !b.ForwardPathname {
			stripKey := "strip-" + routerKey
			if cfg.HTTP.Middlewares == nil {
				cfg.HTTP.Middlewares = make(map[string]traefikMiddleware)
			}
			cfg.HTTP.Middlewares[stripKey] = traefikMiddleware{
				StripPrefix: &traefikStripPrefix{
					Prefixes: []string{b.Path},
				},
			}
			router.Middlewares = append(router.Middlewares, stripKey)
		}

		if b.Public {
			// Public (cloudflare) bindings: HTTP only, no TLS, no redirect
			router.EntryPoints = []string{"web"}
		} else if b.HTTPS && hasCerts {
			router.EntryPoints = []string{"websecure"}
			router.TLS = &traefikRouterTLS{}

			// Add HTTP->HTTPS redirect router
			if cfg.HTTP.Middlewares == nil {
				cfg.HTTP.Middlewares = make(map[string]traefikMiddleware)
			}
			if _, exists := cfg.HTTP.Middlewares["redirect-to-https"]; !exists {
				cfg.HTTP.Middlewares["redirect-to-https"] = traefikMiddleware{
					RedirectScheme: &traefikRedirectScheme{
						Scheme:    "https",
						Permanent: true,
					},
				}
			}
			httpRouter := traefikRouter{
				Rule:        rule,
				Service:     serviceKey,
				EntryPoints: []string{"web"},
				Middlewares: []string{"redirect-to-https"},
			}
			cfg.HTTP.Routers[routerKey+"-http"] = httpRouter
		} else {
			router.EntryPoints = []string{"web"}
		}

		cfg.HTTP.Routers[routerKey] = router

		// Shared service — only create if not already present
		if _, exists := cfg.HTTP.Services[serviceKey]; !exists {
			cfg.HTTP.Services[serviceKey] = traefikService{
				LoadBalancer: traefikLB{
					Servers: []traefikServer{
						{URL: fmt.Sprintf("http://%s:%d%s", b.Container, b.Port, b.TargetPath)},
					},
				},
			}
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
