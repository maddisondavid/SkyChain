package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/skychain/skychain/pkg/registry"
)

type auditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Action      string    `json:"action"`
	DeviceID    string    `json:"device_id"`
	OldKey      string    `json:"old_key,omitempty"`
	NewKey      string    `json:"new_key,omitempty"`
	Description string    `json:"description,omitempty"`
}

func main() {
	devicesPath := flag.String("devices", "config/devices.json", "path to devices.json")
	auditPath := flag.String("audit", "config/devices.audit.log", "path to the registry audit log")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "add", "update":
		if len(args) < 3 {
			log.Fatalf("usage: devicectl %s <device_id> <base64 public key>", command)
		}
		deviceID := args[1]
		newKey := args[2]
		if err := addOrUpdate(*devicesPath, *auditPath, deviceID, newKey, command == "update"); err != nil {
			log.Fatalf("%s device: %v", command, err)
		}
	case "revoke":
		if len(args) < 2 {
			log.Fatalf("usage: devicectl revoke <device_id>")
		}
		if err := revoke(*devicesPath, *auditPath, args[1]); err != nil {
			log.Fatalf("revoke device: %v", err)
		}
	case "list":
		if err := listDevices(*devicesPath); err != nil {
			log.Fatalf("list devices: %v", err)
		}
	default:
		log.Fatalf("unknown command %s", command)
	}
}

func usage() {
	fmt.Println("SkyChain device registry controller")
	fmt.Println("Usage: devicectl [flags] <command>")
	fmt.Println("Commands:")
	fmt.Println("  add <device_id> <base64 public key>    Add a new device entry")
	fmt.Println("  update <device_id> <base64 public key> Update an existing device's key")
	fmt.Println("  revoke <device_id>                     Remove a device from the registry")
	fmt.Println("  list                                   Show the current registry contents")
	flag.PrintDefaults()
}

func addOrUpdate(devicesPath, auditPath, deviceID, newKey string, expectExisting bool) error {
	if deviceID == "" {
		return errors.New("device id required")
	}
	if _, err := registry.ValidatePublicKey(newKey); err != nil {
		return err
	}

	entries, err := readRegistry(devicesPath)
	if err != nil {
		return err
	}

	_, exists := entries[deviceID]
	if expectExisting && !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}
	if !expectExisting && exists {
		return fmt.Errorf("device %s already exists", deviceID)
	}

	oldKey := entries[deviceID]
	entries[deviceID] = newKey

	if err := writeRegistry(devicesPath, entries); err != nil {
		return err
	}

	action := "add"
	if expectExisting {
		action = "update"
	}
	entry := auditEntry{
		Timestamp: time.Now().UTC(),
		Action:    action,
		DeviceID:  deviceID,
		OldKey:    oldKey,
		NewKey:    newKey,
	}
	return appendAudit(auditPath, entry)
}

func revoke(devicesPath, auditPath, deviceID string) error {
	if deviceID == "" {
		return errors.New("device id required")
	}
	entries, err := readRegistry(devicesPath)
	if err != nil {
		return err
	}
	key, exists := entries[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	delete(entries, deviceID)
	if err := writeRegistry(devicesPath, entries); err != nil {
		return err
	}

	entry := auditEntry{
		Timestamp:   time.Now().UTC(),
		Action:      "revoke",
		DeviceID:    deviceID,
		OldKey:      key,
		Description: "removed from registry",
	}
	return appendAudit(auditPath, entry)
}

func listDevices(devicesPath string) error {
	entries, err := readRegistry(devicesPath)
	if err != nil {
		return err
	}

	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		fmt.Printf("%s\t%s\n", id, entries[id])
	}
	if len(ids) == 0 {
		fmt.Println("registry is empty")
	}
	return nil
}

func readRegistry(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return entries, nil
}

func writeRegistry(path string, entries map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "devices-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer tmp.Close()

	if err := registry.Marshal(entries, tmp); err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace registry: %w", err)
	}

	return nil
}

func appendAudit(path string, entry auditEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create audit directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode audit entry: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write audit entry: %w", err)
	}
	return nil
}
