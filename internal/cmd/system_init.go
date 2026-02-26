package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ivancerovina/forge/internal/ui"
)

func forgeComposeYAML(traefikDir, certsDir string) string {
	return `services:
  traefik:
    image: traefik:latest
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

networks:
  forge-network:
    external: true
`
}

func SystemInitHelp() {
	fmt.Println(ui.TitleStyle.Render("forge init") + ui.DescStyle.Render(" - initialize forge system infrastructure"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Usage:"))
	fmt.Println("  " + ui.CmdStyle.Render("forge init") + "  " + ui.DescStyle.Render("Create Docker network and start Traefik"))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Flags:"))
	fmt.Println("  " + ui.CmdStyle.Render("--help") + "  " + ui.DescStyle.Render("Show this help message"))
	fmt.Println()
	fmt.Println(ui.DescStyle.Render("Sets up the shared forge infrastructure:"))
	fmt.Println("  " + ui.DescStyle.Render("• Docker network ") + ui.CmdStyle.Render("forge-network"))
	fmt.Println("  " + ui.DescStyle.Render("• Traefik reverse proxy on ports 80/443"))
}

func generateCerts() bool {
	if _, err := exec.LookPath("mkcert"); err != nil {
		fmt.Println("  " + ui.WarningStyle.Render("!") + " " + ui.CmdStyle.Render("mkcert") + " " + ui.DescStyle.Render("not found — skipping local certificates"))
		fmt.Println("    " + ui.DescStyle.Render("Install from: ") + ui.CmdStyle.Render("https://github.com/FiloSottile/mkcert"))
		return false
	}

	home, err := UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.ErrStyle.Render("could not determine home directory: "+err.Error()))
		return false
	}

	certsDir := filepath.Join(home, ".forge", "certs")
	certPath := filepath.Join(certsDir, "local.pem")
	keyPath := filepath.Join(certsDir, "local-key.pem")

	// Skip if certs already exist
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render("local certificates") + " " + ui.DescStyle.Render("already exist"))
			return true
		}
	}

	// Install local CA (idempotent)
	install := exec.Command("mkcert", "-install")
	install.Stdout = nil
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.CmdStyle.Render("mkcert -install")+" "+ui.ErrStyle.Render("failed: "+err.Error()))
		return false
	}

	// Generate wildcard cert for *.local
	gen := exec.Command("mkcert", "-cert-file", certPath, "-key-file", keyPath, "*.local")
	gen.Stdout = nil
	gen.Stderr = os.Stderr
	if err := gen.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.CmdStyle.Render("mkcert")+" "+ui.ErrStyle.Render("failed to generate certs: "+err.Error()))
		return false
	}

	fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render("local certificates") + " " + ui.SuccessStyle.Render("generated"))
	return true
}

// RegenerateCerts regenerates the local certificates with per-project wildcards
// collected from all registered projects. Called from bind to cover two-level subdomains.
func RegenerateCerts() error {
	if _, err := exec.LookPath("mkcert"); err != nil {
		return nil // silently skip if mkcert not installed
	}

	home, err := UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	certsDir := filepath.Join(home, ".forge", "certs")
	certPath := filepath.Join(certsDir, "local.pem")
	keyPath := filepath.Join(certsDir, "local-key.pem")

	// Collect domains: start with *.local
	domains := []string{"*.local"}

	// Add *.<code>.local for each registered project
	paths, err := readProjects()
	if err == nil {
		for _, p := range paths {
			proj, err := readForgeRCAt(p)
			if err != nil {
				continue
			}
			domains = append(domains, "*."+proj.Code+".local")
		}
	}

	// Run mkcert -install (idempotent)
	install := exec.Command("mkcert", "-install")
	install.Stdout = nil
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("mkcert -install failed: %w", err)
	}

	args := []string{"-cert-file", certPath, "-key-file", keyPath}
	args = append(args, domains...)
	gen := exec.Command("mkcert", args...)
	gen.Stdout = nil
	gen.Stderr = os.Stderr
	if err := gen.Run(); err != nil {
		return fmt.Errorf("mkcert failed to generate certs: %w", err)
	}

	return nil
}

func writeTLSConfig() bool {
	home, err := UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.ErrStyle.Render("could not determine home directory: "+err.Error()))
		return false
	}

	tlsConfig := `tls:
  stores:
    default:
      defaultCertificate:
        certFile: /etc/traefik/certs/local.pem
        keyFile: /etc/traefik/certs/local-key.pem
`
	path := filepath.Join(home, ".forge", "traefik", "_tls.yml")
	if err := os.WriteFile(path, []byte(tlsConfig), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.CmdStyle.Render("_tls.yml")+" "+ui.ErrStyle.Render("failed to write: "+err.Error()))
		return false
	}

	fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render("~/.forge/traefik/_tls.yml") + " " + ui.SuccessStyle.Render("written"))
	return true
}

func SystemInit(args []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			SystemInitHelp()
			os.Exit(0)
		}
	}

	fmt.Println(ui.TitleStyle.Render("Initializing forge system..."))
	fmt.Println()

	// 1. Ensure forge-network exists
	networkOk := ensureNetwork()

	// 2. Generate local certificates (non-fatal)
	certsOk := generateCerts()
	if certsOk {
		writeTLSConfig()
	}

	// 3. Write compose file to ~/.forge/
	composeOk := writeComposeFile()

	// 4. Start Traefik
	traefikOk := startTraefik()

	// Summary
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Summary:"))
	printStatus("forge-network", networkOk)
	printStatus("local certificates", certsOk)
	printStatus("forge-traefik", traefikOk && composeOk)

	if !networkOk || !composeOk || !traefikOk {
		os.Exit(1)
	}
}

func ensureNetwork() bool {
	// Check if network already exists
	check := exec.Command("docker", "network", "inspect", "forge-network")
	check.Stdout = nil
	check.Stderr = nil
	if check.Run() == nil {
		fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render("forge-network") + " " + ui.DescStyle.Render("already exists"))
		return true
	}

	// Create it
	create := exec.Command("docker", "network", "create", "forge-network")
	create.Stdout = nil
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.CmdStyle.Render("forge-network")+" "+ui.ErrStyle.Render("failed to create: "+err.Error()))
		return false
	}

	fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render("forge-network") + " " + ui.SuccessStyle.Render("created"))
	return true
}

func writeComposeFile() bool {
	home, err := UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.ErrStyle.Render("could not determine home directory: "+err.Error()))
		return false
	}

	traefikDir := filepath.Join(home, ".forge", "traefik")
	certsDir := filepath.Join(home, ".forge", "certs")
	path := filepath.Join(home, ".forge", "docker-compose.yml")
	if err := os.WriteFile(path, []byte(forgeComposeYAML(traefikDir, certsDir)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.CmdStyle.Render("docker-compose.yml")+" "+ui.ErrStyle.Render("failed to write: "+err.Error()))
		return false
	}

	fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render("~/.forge/docker-compose.yml") + " " + ui.SuccessStyle.Render("written"))
	return true
}

func startTraefik() bool {
	home, err := UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.ErrStyle.Render("could not determine home directory: "+err.Error()))
		return false
	}

	composePath := filepath.Join(home, ".forge", "docker-compose.yml")
	up := exec.Command("docker", "compose", "-f", composePath, "up", "-d")
	up.Stdout = nil
	up.Stderr = os.Stderr
	if err := up.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "  "+ui.ErrStyle.Render("✗")+" "+ui.CmdStyle.Render("forge-traefik")+" "+ui.ErrStyle.Render("failed to start: "+err.Error()))
		return false
	}

	fmt.Println("  " + ui.SuccessStyle.Render("✓") + " " + ui.CmdStyle.Render("forge-traefik") + " " + ui.SuccessStyle.Render("started"))
	return true
}

func printStatus(name string, ok bool) {
	if ok {
		fmt.Println("  " + ui.SuccessStyle.Render("●") + " " + ui.CmdStyle.Render(name) + "  " + ui.SuccessStyle.Render("ready"))
	} else {
		fmt.Println("  " + ui.ErrStyle.Render("●") + " " + ui.CmdStyle.Render(name) + "  " + ui.ErrStyle.Render("failed"))
	}
}
