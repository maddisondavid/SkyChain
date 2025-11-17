package registry

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"sync"
	"time"
)

// DeviceRecord represents a single device entry in the registry.
type DeviceRecord struct {
	ID        string
	PublicKey string
	keyBytes  []byte
	UpdatedAt time.Time
}

// DeviceRegistry stores the currently authorized devices and their keys.
type DeviceRegistry struct {
	path string

	mu      sync.RWMutex
	devices map[string]DeviceRecord
	loaded  time.Time
}

// ErrUnknownDevice is returned when a lookup is performed for a device that is
// not registered or has been revoked.
var ErrUnknownDevice = errors.New("device not registered")

// Load creates a registry by reading the provided JSON file.
func Load(path string) (*DeviceRegistry, error) {
	if path == "" {
		return nil, errors.New("registry path required")
	}

	devices, modTime, err := readRegistryFile(path)
	if err != nil {
		return nil, err
	}

	return &DeviceRegistry{
		path:    path,
		devices: devices,
		loaded:  modTime,
	}, nil
}

// Path returns the file backing the registry.
func (r *DeviceRegistry) Path() string {
	return r.path
}

// Size returns the number of active devices in the registry.
func (r *DeviceRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.devices)
}

// Lookup returns the device record for the provided ID.
func (r *DeviceRegistry) Lookup(id string) (DeviceRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.devices[id]
	if !ok {
		return DeviceRecord{}, ErrUnknownDevice
	}
	return record, nil
}

// PublicKey returns the decoded ED25519 public key for the device.
func (r *DeviceRegistry) PublicKey(id string) ([]byte, error) {
	record, err := r.Lookup(id)
	if err != nil {
		return nil, err
	}
	key := make([]byte, len(record.keyBytes))
	copy(key, record.keyBytes)
	return key, nil
}

// Reload refreshes the registry from disk.
func (r *DeviceRegistry) Reload() error {
	devices, modTime, err := readRegistryFile(r.path)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.devices = devices
	r.loaded = modTime
	r.mu.Unlock()
	return nil
}

// Watch periodically polls the registry file for changes and reloads the
// in-memory structure when modifications occur. Watch blocks until the
// context is cancelled.
func (r *DeviceRegistry) Watch(ctx context.Context, logger *log.Logger) error {
	const pollInterval = 3 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			stat, err := os.Stat(r.path)
			if err != nil {
				if logger != nil {
					logger.Printf("device registry stat failed: %v", err)
				}
				continue
			}
			r.mu.RLock()
			last := r.loaded
			r.mu.RUnlock()
			if stat.ModTime().After(last) {
				if err := r.Reload(); err != nil {
					if logger != nil {
						logger.Printf("device registry reload failed: %v", err)
					}
					continue
				}
				if logger != nil {
					logger.Printf("device registry reloaded from %s", r.path)
				}
			}
		}
	}
}

func readRegistryFile(path string) (map[string]DeviceRecord, time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("open registry: %w", err)
	}
	defer f.Close()

	devices, err := decodeRegistry(f)
	if err != nil {
		return nil, time.Time{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stat registry: %w", err)
	}
	return devices, info.ModTime(), nil
}

func decodeRegistry(r io.Reader) (map[string]DeviceRecord, error) {
	rawMap, err := parseStrictMap(r)
	if err != nil {
		return nil, err
	}

	devices := make(map[string]DeviceRecord, len(rawMap))
	for id, key := range rawMap {
		decoded, err := ValidatePublicKey(key)
		if err != nil {
			return nil, fmt.Errorf("device %s: %w", id, err)
		}
		devices[id] = DeviceRecord{ID: id, PublicKey: key, keyBytes: decoded, UpdatedAt: time.Now().UTC()}
	}
	return devices, nil
}

func parseStrictMap(r io.Reader) (map[string]string, error) {
	dec := json.NewDecoder(r)
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, errors.New("registry must be a JSON object")
	}

	result := make(map[string]string)
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, errors.New("registry keys must be strings")
		}
		var value string
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("parse value for %s: %w", key, err)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate device id %s", key)
		}
		result[key] = value
	}

	endTok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parse registry end: %w", err)
	}
	endDelim, ok := endTok.(json.Delim)
	if !ok || endDelim != '}' {
		return nil, errors.New("registry must end with closing brace")
	}

	return result, nil
}

// ValidatePublicKey ensures the supplied base64 value decodes into a valid
// ED25519 public key.
func ValidatePublicKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, errors.New("public key required")
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	if l := len(decoded); l != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d byte key, got %d", ed25519.PublicKeySize, l)
	}
	out := make([]byte, len(decoded))
	copy(out, decoded)
	return out, nil
}

// Marshal writes the registry to the provided writer using deterministic key
// ordering. This helper is primarily intended for tooling that modifies the
// registry file while maintaining a predictable structure.
func Marshal(devices map[string]string, w io.Writer) error {
	if devices == nil {
		devices = map[string]string{}
	}
	buf := &bytes.Buffer{}
	buf.WriteByte('{')

	keys := make([]string, 0, len(devices))
	for id := range devices {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	for i, id := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyJSON, _ := json.Marshal(id)
		buf.Write(keyJSON)
		buf.WriteByte(':')
		valueJSON, _ := json.Marshal(devices[id])
		buf.Write(valueJSON)
	}

	buf.WriteByte('}')
	_, err := w.Write(buf.Bytes())
	return err
}
