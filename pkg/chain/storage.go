package chain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SaveToFile persists the chain to the provided path as JSON.
func (c *Chain) SaveToFile(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if path == "" {
		return errors.New("path required")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "chain-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(c.blocks); err != nil {
		return fmt.Errorf("encode chain: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync chain: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return fmt.Errorf("rename chain file: %w", err)
	}

	return nil
}

// LoadOrCreate loads a chain from the provided path. If the file does not
// exist, a new chain is created and persisted before returning.
func LoadOrCreate(path, validator, secret string) (*Chain, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		chain, err := NewChain(validator, secret)
		if err != nil {
			return nil, err
		}
		if err := chain.SaveToFile(path); err != nil {
			return nil, err
		}
		return chain, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open chain file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read chain file: %w", err)
	}

	var blocks []Block
	if err := json.Unmarshal(data, &blocks); err != nil {
		return nil, fmt.Errorf("decode chain: %w", err)
	}

	chain, err := NewChain(validator, secret)
	if err != nil {
		return nil, err
	}

	if err := chain.ReplaceBlocks(blocks); err != nil {
		return nil, err
	}

	return chain, nil
}
