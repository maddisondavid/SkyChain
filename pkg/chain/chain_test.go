package chain

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendBlockLinkage(t *testing.T) {
	chain, err := NewChain("validator", "secret")
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}

	evt := Event{
		DeviceID: "sensor-1",
		Nonce:    1,
		TS:       time.Now().UTC(),
		Payload: map[string]interface{}{
			"temp": 21.5,
		},
	}

	block, err := chain.AppendBlock([]Event{evt})
	if err != nil {
		t.Fatalf("append block: %v", err)
	}

	if block.Index != 1 {
		t.Fatalf("expected block index 1 got %d", block.Index)
	}

	head := chain.Head()
	if head.PrevHash != chain.blocks[0].Hash {
		t.Fatalf("head prev hash mismatch")
	}

	if err := chain.Validate(); err != nil {
		t.Fatalf("chain validate: %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	chain, err := NewChain("validator", "secret")
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}

	evt := Event{DeviceID: "sensor-2", Nonce: 1, TS: time.Now().UTC()}
	if _, err := chain.AppendBlock([]Event{evt}); err != nil {
		t.Fatalf("append block: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")

	if err := chain.SaveToFile(path); err != nil {
		t.Fatalf("save chain: %v", err)
	}

	loaded, err := LoadOrCreate(path, "validator", "secret")
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}

	if loaded.Length() != chain.Length() {
		t.Fatalf("expected %d blocks got %d", chain.Length(), loaded.Length())
	}

	if err := loaded.Validate(); err != nil {
		t.Fatalf("loaded chain invalid: %v", err)
	}

	// ensure the file remains after load
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected chain file to exist: %v", err)
	}
}
