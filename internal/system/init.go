package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ivancerovina/forge/internal/config"
)

type StepResult struct {
	Name    string // "forge-network", "local certificates", "forge-traefik"
	OK      bool
	Message string // "already exists", "created", "failed: ..."
}

type InitResult struct {
	Steps []StepResult
}

// Init orchestrates all system initialization steps and returns the results.
func Init() (*InitResult, error) {
	forgeDir, err := config.ForgeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine forge directory: %w", err)
	}

	result := &InitResult{}

	// 1. Ensure forge-network exists
	msg, err := EnsureNetwork()
	result.Steps = append(result.Steps, StepResult{
		Name:    "forge-network",
		OK:      err == nil,
		Message: msgOrErr(msg, err),
	})

	// 2. Generate local certificates (non-fatal)
	certsMsg, certsErr := GenerateCerts(forgeDir)
	result.Steps = append(result.Steps, StepResult{
		Name:    "local certificates",
		OK:      certsErr == nil,
		Message: msgOrErr(certsMsg, certsErr),
	})

	if certsErr == nil {
		_ = WriteTLSConfig(forgeDir)
	}

	// 3. Write compose file + start Traefik
	composeErr := WriteComposeFile(forgeDir)
	traefikErr := StartTraefik(forgeDir)

	traefikOK := composeErr == nil && traefikErr == nil
	var traefikMsg string
	if traefikOK {
		traefikMsg = "started"
	} else if composeErr != nil {
		traefikMsg = composeErr.Error()
	} else {
		traefikMsg = traefikErr.Error()
	}

	result.Steps = append(result.Steps, StepResult{
		Name:    "forge-traefik",
		OK:      traefikOK,
		Message: traefikMsg,
	})

	// 4. Report cloudflared status (if tunnel enabled)
	cfg, _ := config.ReadConfig()
	if cfg.CloudflareTunnel {
		cfMsg := traefikMsg // same compose stack
		if traefikOK {
			cfMsg = "started"
		}
		result.Steps = append(result.Steps, StepResult{
			Name:    "forge-cloudflared",
			OK:      traefikOK,
			Message: cfMsg,
		})
	}

	return result, nil
}

func msgOrErr(msg string, err error) string {
	if err != nil {
		return err.Error()
	}
	return msg
}

// EnsureNetwork creates the forge-network Docker network if it doesn't exist.
func EnsureNetwork() (string, error) {
	check := exec.Command("docker", "network", "inspect", "forge-network")
	check.Stdout = nil
	check.Stderr = nil
	if check.Run() == nil {
		return "already exists", nil
	}

	create := exec.Command("docker", "network", "create", "--subnet=10.100.0.0/16", "forge-network")
	create.Stdout = nil
	create.Stderr = nil
	if err := create.Run(); err != nil {
		return "", fmt.Errorf("failed to create: %w", err)
	}

	return "created", nil
}

// GenerateCerts generates local TLS certificates using mkcert.
// Returns a status message or error. Does not print to stdout/stderr.
func GenerateCerts(forgeDir string) (string, error) {
	if _, err := exec.LookPath("mkcert"); err != nil {
		return "", fmt.Errorf("mkcert not found — install from https://github.com/FiloSottile/mkcert")
	}

	certsDir := filepath.Join(forgeDir, "certs")
	certPath := filepath.Join(certsDir, "local.pem")
	keyPath := filepath.Join(certsDir, "local-key.pem")

	// Skip if certs already exist
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return "already exist", nil
		}
	}

	// Install local CA (idempotent)
	install := exec.Command("mkcert", "-install")
	install.Stdout = nil
	install.Stderr = nil
	if err := install.Run(); err != nil {
		return "", fmt.Errorf("mkcert -install failed: %w", err)
	}

	// Generate wildcard cert for *.test
	gen := exec.Command("mkcert", "-cert-file", certPath, "-key-file", keyPath, "*.test")
	gen.Stdout = nil
	gen.Stderr = nil
	if err := gen.Run(); err != nil {
		return "", fmt.Errorf("mkcert failed to generate certs: %w", err)
	}

	return "generated", nil
}

// RegenerateCerts regenerates the local certificates with per-project wildcards
// collected from all registered projects. Called from bind to cover two-level subdomains.
// Does not print to stdout/stderr.
func RegenerateCerts() error {
	if _, err := exec.LookPath("mkcert"); err != nil {
		return nil // silently skip if mkcert not installed
	}

	forgeDir, err := config.ForgeDir()
	if err != nil {
		return fmt.Errorf("could not determine forge directory: %w", err)
	}

	certsDir := filepath.Join(forgeDir, "certs")
	certPath := filepath.Join(certsDir, "local.pem")
	keyPath := filepath.Join(certsDir, "local-key.pem")

	// Collect domains: start with *.test
	domains := []string{"*.test"}

	// Add <code>.test and *.<code>.test for each registered project
	// Both are needed: wildcards don't cover the bare domain.
	paths, err := config.ReadProjects()
	if err == nil {
		for _, p := range paths {
			proj, err := config.ReadForgeRC(p)
			if err != nil {
				continue
			}
			domains = append(domains, proj.Code+".test", "*."+proj.Code+".test")
		}
	}

	// Run mkcert -install (idempotent)
	install := exec.Command("mkcert", "-install")
	install.Stdout = nil
	install.Stderr = nil
	if err := install.Run(); err != nil {
		return fmt.Errorf("mkcert -install failed: %w", err)
	}

	args := []string{"-cert-file", certPath, "-key-file", keyPath}
	args = append(args, domains...)
	gen := exec.Command("mkcert", args...)
	gen.Stdout = nil
	gen.Stderr = nil
	if err := gen.Run(); err != nil {
		return fmt.Errorf("mkcert failed to generate certs: %w", err)
	}

	return nil
}

// WriteTLSConfig writes the Traefik TLS config file.
func WriteTLSConfig(forgeDir string) error {
	tlsConfig := `tls:
  stores:
    default:
      defaultCertificate:
        certFile: /etc/traefik/certs/local.pem
        keyFile: /etc/traefik/certs/local-key.pem
`
	path := filepath.Join(forgeDir, "traefik", "_tls.yml")
	if err := os.WriteFile(path, []byte(tlsConfig), 0o644); err != nil {
		return fmt.Errorf("failed to write _tls.yml: %w", err)
	}
	return nil
}

// WriteCFConfig writes the cloudflared ingress config to ~/.forge/cf-config.yml.
func WriteCFConfig(forgeDir string) error {
	cfConfig := `ingress:
  - service: http://forge-traefik:80
`
	path := filepath.Join(forgeDir, "cf-config.yml")
	if err := os.WriteFile(path, []byte(cfConfig), 0o644); err != nil {
		return fmt.Errorf("failed to write cf-config.yml: %w", err)
	}
	return nil
}

func forgeComposeYAML(forgeDir string, tunnelEnabled bool) string {
	traefikDir := filepath.Join(forgeDir, "traefik")
	certsDir := filepath.Join(forgeDir, "certs")

	yaml := `services:
  traefik:
    image: traefik:v3.3
    container_name: forge-traefik
    restart: unless-stopped
    command:
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--providers.file.directory=/etc/traefik/dynamic"
      - "--providers.file.watch=true"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ` + traefikDir + `:/etc/traefik/dynamic:ro
      - ` + certsDir + `:/etc/traefik/certs:ro
    networks:
      - forge-network
`

	if tunnelEnabled {
		cfConfigPath := filepath.Join(forgeDir, "cf-config.yml")
		yaml += `
  cloudflared:
    image: cloudflare/cloudflared:latest
    container_name: forge-cloudflared
    restart: unless-stopped
    command: tunnel --config /etc/cloudflared/config.yml run --token ${CLOUDFLARE_TUNNEL_TOKEN}
    volumes:
      - ` + cfConfigPath + `:/etc/cloudflared/config.yml:ro
    networks:
      - forge-network
    depends_on:
      - traefik
`
	}

	yaml += `
networks:
  forge-network:
    external: true
`
	return yaml
}

// WriteComposeFile writes the system-level docker-compose.yml to the forge directory.
func WriteComposeFile(forgeDir string) error {
	path := filepath.Join(forgeDir, "docker-compose.yml")

	cfg, _ := config.ReadConfig() // ignore error: missing config = no tunnel

	if err := os.WriteFile(path, []byte(forgeComposeYAML(forgeDir, cfg.CloudflareTunnel)), 0o644); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}
	return nil
}

// StartTraefik starts the forge system stack (Traefik + optional cloudflared) via docker compose.
func StartTraefik(forgeDir string) error {
	composePath := filepath.Join(forgeDir, "docker-compose.yml")
	up := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--remove-orphans")
	up.Stdout = nil
	up.Stderr = nil
	if err := up.Run(); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}
	return nil
}

// CertsAvailable checks if the local TLS certificates exist.
func CertsAvailable() bool {
	forgeDir, err := config.ForgeDir()
	if err != nil {
		return false
	}
	certPath := filepath.Join(forgeDir, "certs", "local.pem")
	if _, err := os.Stat(certPath); err != nil {
		return false
	}
	return true
}
