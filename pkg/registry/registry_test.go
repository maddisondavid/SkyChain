package registry

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	data := []byte(`{"sensor-1":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","sensor-2":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	reg, err := Load(path)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}

	if _, err := reg.PublicKey("sensor-1"); err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got := reg.Size(); got != 2 {
		t.Fatalf("size mismatch: %d", got)
	}
}

func TestDuplicateDetection(t *testing.T) {
	data := []byte(`{"sensor-1":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","sensor-1":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="}`)
	if _, err := decodeRegistry(bytes.NewReader(data)); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestInvalidKey(t *testing.T) {
	data := []byte(`{"sensor-1":"not-base64"}`)
	if _, err := decodeRegistry(bytes.NewReader(data)); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devices.json")
	initial := []byte(`{"sensor-1":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}`)
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	reg, err := Load(path)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}

	if _, err := reg.Lookup("sensor-1"); err != nil {
		t.Fatalf("lookup existing: %v", err)
	}

	updated := []byte(`{"sensor-2":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="}`)
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatalf("update registry: %v", err)
	}

	if err := reg.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if _, err := reg.Lookup("sensor-2"); err != nil {
		t.Fatalf("lookup updated: %v", err)
	}
	if _, err := reg.Lookup("sensor-1"); err == nil {
		t.Fatal("expected sensor-1 to be removed")
	}
}
