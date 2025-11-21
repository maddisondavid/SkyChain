package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/skychain/skychain/pkg/chain"
)

// BlockStore defines persistence capabilities required by the node.
type BlockStore interface {
	// LoadChain restores the in-memory chain from the store. A new chain is
	// created and persisted when the store is empty.
	LoadChain(validator, secret string) (*chain.Chain, error)
	// PersistBlock appends the provided block to durable storage.
	PersistBlock(block chain.Block) error
	// Close releases any underlying resources.
	Close() error
}

// FileBlockStore stores blocks in a newline-delimited JSON log.
type FileBlockStore struct {
	path string
	mu   sync.Mutex
}

// OpenFileBlockStore creates a store backed by the provided path.
func OpenFileBlockStore(path string) (*FileBlockStore, error) {
	if path == "" {
		return nil, errors.New("path required")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	return &FileBlockStore{path: path}, nil
}

// Close implements BlockStore. File stores keep no open handles so this is a no-op.
func (s *FileBlockStore) Close() error { return nil }

// LoadChain reconstructs the chain from the JSONL log or writes a genesis block when empty.
func (s *FileBlockStore) LoadChain(validator, secret string) (*chain.Chain, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}

	ledger, err := chain.NewChain(validator, secret)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(s.path, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open block log: %w", err)
	}
	defer file.Close()

	blocks := make([]chain.Block, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var blk chain.Block
		if err := json.Unmarshal(line, &blk); err != nil {
			return nil, fmt.Errorf("decode block: %w", err)
		}
		blocks = append(blocks, blk)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan block log: %w", err)
	}

	if len(blocks) == 0 {
		if err := s.persistAll(ledger.Blocks()); err != nil {
			return nil, err
		}
		return ledger, nil
	}

	if err := ledger.ReplaceBlocks(blocks); err != nil {
		return nil, err
	}

	return ledger, nil
}

// PersistBlock appends a JSON line to the store.
func (s *FileBlockStore) PersistBlock(block chain.Block) error {
	if s == nil {
		return errors.New("store is nil")
	}

	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("encode block: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open block log: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write block: %w", err)
	}
	return f.Sync()
}

func (s *FileBlockStore) persistAll(blocks []chain.Block) error {
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "chain-*.jsonl")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpName)
		}
	}()

	writer := bufio.NewWriter(tmp)
	for _, block := range blocks {
		data, err := json.Marshal(block)
		if err != nil {
			tmp.Close()
			return fmt.Errorf("encode block: %w", err)
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			tmp.Close()
			return fmt.Errorf("write block: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		tmp.Close()
		return fmt.Errorf("flush temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace block log: %w", err)
	}
	cleanup = false
	return nil
}
