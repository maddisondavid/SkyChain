package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

func TestBlockCarriesMerkleRoot(t *testing.T) {
	chain, err := NewChain("validator", "secret")
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}

	evt := Event{DeviceID: "sensor-3", Nonce: 2, TS: time.Unix(0, 0).UTC()}
	block, err := chain.AppendBlock([]Event{evt})
	if err != nil {
		t.Fatalf("append block: %v", err)
	}

	root, err := computeMerkleRoot(block.Events)
	if err != nil {
		t.Fatalf("compute merkle root: %v", err)
	}
	if block.MerkleRoot != root {
		t.Fatalf("expected block merkle root %s got %s", root, block.MerkleRoot)
	}
}

func TestComputeMerkleRootDeterministic(t *testing.T) {
	evt := Event{
		DeviceID: "sensor-5",
		Nonce:    10,
		TS:       time.Unix(100, 0).UTC(),
		Payload: map[string]any{
			"temp": 42.1,
		},
		Signature: "sig",
	}

	root, err := computeMerkleRoot([]Event{evt})
	if err != nil {
		t.Fatalf("compute merkle root: %v", err)
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	sum := sha256.Sum256(data)
	expected := hex.EncodeToString(sum[:])
	if root != expected {
		t.Fatalf("expected %s got %s", expected, root)
	}
}

func TestChainValidateDetectsMerkleMismatch(t *testing.T) {
	chain, err := NewChain("validator", "secret")
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}

	evt := Event{DeviceID: "sensor-6", Nonce: 1, TS: time.Now().UTC()}
	if _, err := chain.AppendBlock([]Event{evt}); err != nil {
		t.Fatalf("append block: %v", err)
	}

	if len(chain.blocks) < 2 {
		t.Fatalf("expected chain to have at least 2 blocks")
	}
	chain.blocks[1].MerkleRoot = "deadbeef"

	if err := chain.Validate(); err == nil {
		t.Fatalf("expected validation to fail for tampered merkle root")
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

func TestReplaceBlocks(t *testing.T) {
	chain, err := NewChain("validator", "secret")
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}

	newBlocks := make([]Block, 3)
	for i := 0; i < 3; i++ {
		prevHash := ""
		if i > 0 {
			prevHash = newBlocks[i-1].Hash
		}
		block := Block{
			Index:     i,
			Timestamp: time.Now().UTC(),
			PrevHash:  prevHash,
			Events:    []Event{},
			Validator: "validator",
		}
		root, err := computeMerkleRoot(block.Events)
		if err != nil {
			t.Fatalf("compute merkle root: %v", err)
		}
		block.MerkleRoot = root
		hash, err := computeBlockHash(block)
		if err != nil {
			t.Fatalf("compute block hash: %v", err)
		}
		block.Hash = hash
		block.Signature = signHash(hash, "secret")
		newBlocks[i] = block
	}

	if err := chain.ReplaceBlocks(newBlocks); err != nil {
		t.Fatalf("replace blocks: %v", err)
	}

	if chain.Length() != 3 {
		t.Fatalf("expected chain length 3 got %d", chain.Length())
	}

	if err := chain.Validate(); err != nil {
		t.Fatalf("chain validate: %v", err)
	}
}