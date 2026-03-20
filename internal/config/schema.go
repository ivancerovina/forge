package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed forgerc.schema.json
var forgeRCSchema []byte

// SchemaURI returns the canonical URL for the forge JSON Schema.
func SchemaURI() string {
	return "https://raw.githubusercontent.com/ivancerovina/forge/refs/heads/master/internal/config/forgerc.schema.json"
}

// EnsureSchema writes the embedded JSON Schema to ~/.forge/schemas/forgerc.schema.json.
// Always overwrites to keep the on-disk copy in sync with the binary.
func EnsureSchema() error {
	home, err := UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	dir := filepath.Join(home, ".forge", "schemas")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", dir, err)
	}

	path := filepath.Join(dir, "forgerc.schema.json")
	if err := os.WriteFile(path, forgeRCSchema, 0o644); err != nil {
		return fmt.Errorf("could not write schema: %w", err)
	}
	return nil
}
