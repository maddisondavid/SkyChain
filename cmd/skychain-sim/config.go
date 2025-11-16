package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config defines the simulator configuration parameters loaded from a YAML or JSON file.
type Config struct {
	Devices            int      `json:"devices" yaml:"devices"`
	MinIntervalMS      int      `json:"min_interval_ms" yaml:"min_interval_ms"`
	MaxIntervalMS      int      `json:"max_interval_ms" yaml:"max_interval_ms"`
	OfflineProbability float64  `json:"offline_probability" yaml:"offline_probability"`
	SpikeProbability   float64  `json:"spike_probability" yaml:"spike_probability"`
	Endpoint           string   `json:"endpoint" yaml:"endpoint"`
	Sensors            []string `json:"sensors" yaml:"sensors"`
	filePath           string
}

// LoadConfig reads and validates the simulator configuration from the provided path.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := Config{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		if err := parseYAMLConfig(string(data), &cfg); err != nil {
			return Config{}, fmt.Errorf("decode config: %w", err)
		}
	}

	cfg.filePath = path

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func parseYAMLConfig(content string, cfg *Config) error {
	var parsingList bool
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "-") {
			if !parsingList {
				return fmt.Errorf("unexpected list item: %s", trimmed)
			}
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			value = strings.Trim(value, "\"'")
			cfg.Sensors = append(cfg.Sensors, value)
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid line: %s", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")

		parsingList = false
		switch key {
		case "devices":
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("parse devices: %w", err)
			}
			cfg.Devices = v
		case "min_interval_ms":
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("parse min_interval_ms: %w", err)
			}
			cfg.MinIntervalMS = v
		case "max_interval_ms":
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("parse max_interval_ms: %w", err)
			}
			cfg.MaxIntervalMS = v
		case "offline_probability":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("parse offline_probability: %w", err)
			}
			cfg.OfflineProbability = v
		case "spike_probability":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("parse spike_probability: %w", err)
			}
			cfg.SpikeProbability = v
		case "endpoint":
			cfg.Endpoint = value
		case "sensors":
			cfg.Sensors = nil
			parsingList = true
		default:
			return fmt.Errorf("unknown key: %s", key)
		}
	}

	return nil
}

func (c Config) validate() error {
	if c.Devices <= 0 {
		return errors.New("devices must be > 0")
	}
	if c.MinIntervalMS <= 0 {
		return errors.New("min_interval_ms must be > 0")
	}
	if c.MaxIntervalMS < c.MinIntervalMS {
		return errors.New("max_interval_ms must be >= min_interval_ms")
	}
	if c.OfflineProbability < 0 || c.OfflineProbability > 1 {
		return errors.New("offline_probability must be between 0 and 1")
	}
	if c.SpikeProbability < 0 || c.SpikeProbability > 1 {
		return errors.New("spike_probability must be between 0 and 1")
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return errors.New("endpoint is required")
	}
	if len(c.Sensors) == 0 {
		return errors.New("sensors must contain at least one sensor type")
	}
	return nil
}

// MinInterval returns the minimum interval duration between device readings.
func (c Config) MinInterval() time.Duration {
	return time.Duration(c.MinIntervalMS) * time.Millisecond
}

// MaxInterval returns the maximum interval duration between device readings.
func (c Config) MaxInterval() time.Duration {
	return time.Duration(c.MaxIntervalMS) * time.Millisecond
}

// Source returns the original configuration file path.
func (c Config) Source() string {
	return c.filePath
}
