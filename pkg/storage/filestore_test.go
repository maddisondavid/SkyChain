package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/skychain/skychain/pkg/chain"
)

func TestFileBlockStoreLoadChainNewDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.db")

	store, err := OpenFileBlockStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ledger, err := store.LoadChain("validator", "secret")
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}

	if ledger.Length() != 1 {
		t.Fatalf("expected genesis only got %d blocks", ledger.Length())
	}
}

func TestFileBlockStorePersistsBlocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.db")

	store, err := OpenFileBlockStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	ledger, err := store.LoadChain("validator", "secret")
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}

	evt := chain.Event{DeviceID: "sensor-1", Nonce: 1, TS: time.Unix(0, 0).UTC()}
	block, err := ledger.AppendBlock([]chain.Event{evt})
	if err != nil {
		t.Fatalf("append block: %v", err)
	}

	if err := store.PersistBlock(block); err != nil {
		t.Fatalf("persist block: %v", err)
	}

	store.Close()

	reopened, err := OpenFileBlockStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer reopened.Close()

	restored, err := reopened.LoadChain("validator", "secret")
	if err != nil {
		t.Fatalf("reload chain: %v", err)
	}

	if restored.Length() != 2 {
		t.Fatalf("expected 2 blocks got %d", restored.Length())
	}
}
