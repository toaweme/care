package main

import (
	"testing"

	yamlcodec "github.com/toaweme/cli/config/addons/yaml"

	"github.com/toaweme/care"
	"github.com/toaweme/care/eco/golang"
)

// Test_HealthConfig_OverlaysEcosystemDefaults checks the grading config path: seeding
// cfg.Health with the ecosystem defaults and decoding a care.yml health block overlays
// only the keys the operator set (the rest keep the defaults), and an explicit 0 is
// honored so a feature can be tuned to informational.
func Test_HealthConfig_OverlaysEcosystemDefaults(t *testing.T) {
	cfg := care.Defaults()
	cfg.Health = golang.DefaultRating()

	doc := []byte("health:\n  weights:\n    dependencies: 20\n    docs: 0\n  caps:\n    vulnerabilities: 80\n")
	if err := yamlcodec.New(".yml").Unmarshal(doc, &cfg); err != nil {
		t.Fatalf("decode health config: %v", err)
	}

	w, c := cfg.Health.Weights, cfg.Health.Caps
	if w.Dependencies != 20 {
		t.Errorf("dependencies weight = %d, want overridden 20", w.Dependencies)
	}
	if w.Docs != 0 {
		t.Errorf("docs weight = %d, want overridden 0 (informational)", w.Docs)
	}
	if w.Build != 20 {
		t.Errorf("build weight = %d, want default 20 kept", w.Build)
	}
	if c.Vulnerabilities != 80 {
		t.Errorf("vulnerabilities cap = %d, want overridden 80", c.Vulnerabilities)
	}
	if c.Secrets != 40 {
		t.Errorf("secrets cap = %d, want default 40 kept", c.Secrets)
	}
}
