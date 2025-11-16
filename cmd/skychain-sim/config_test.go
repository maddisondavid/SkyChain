package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`devices: 5
min_interval_ms: 1000
max_interval_ms: 2000
offline_probability: 0.25
spike_probability: 0.1
endpoint: "http://localhost:8080/event"
sensors:
  - temperature
  - humidity
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Devices != 5 {
		t.Fatalf("expected 5 devices, got %d", cfg.Devices)
	}
	if cfg.MinInterval() != time.Second {
		t.Fatalf("expected min interval 1s, got %v", cfg.MinInterval())
	}
	if cfg.MaxInterval() != 2*time.Second {
		t.Fatalf("expected max interval 2s, got %v", cfg.MaxInterval())
	}
	if cfg.Source() != path {
		t.Fatalf("expected source %s, got %s", path, cfg.Source())
	}
}

func TestLoadConfigValidationError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`devices: 0
min_interval_ms: 1000
max_interval_ms: 500
offline_probability: -0.1
spike_probability: 1.2
endpoint: ""
sensors: []
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected validation error but got nil")
	}
}

func TestSelectSensors(t *testing.T) {
	sensors := []string{"temperature", "humidity", "pressure", "wind", "rain"}
	rnd := rand.New(rand.NewSource(42))

	got := selectSensors(rnd, sensors)
	if len(got) < 1 || len(got) > 3 {
		t.Fatalf("expected between 1 and 3 sensors, got %d", len(got))
	}

	seen := make(map[string]struct{})
	for _, s := range got {
		if _, ok := seen[s]; ok {
			t.Fatalf("sensor %s selected multiple times", s)
		}
		seen[s] = struct{}{}
	}
}
