package chain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Event represents a single IoT event that will be persisted on the blockchain.
type Event struct {
	DeviceID string                 `json:"device_id"`
	Nonce    string                 `json:"nonce"`
	TS       time.Time              `json:"ts"`
	Payload  map[string]interface{} `json:"payload"`
}

// Block contains a list of events and links to the previous block via hash.
type Block struct {
	Index     int       `json:"index"`
	Timestamp time.Time `json:"timestamp"`
	PrevHash  string    `json:"prev_hash"`
	Hash      string    `json:"hash"`
	Events    []Event   `json:"events"`
	Validator string    `json:"validator"`
	Signature string    `json:"signature"`
}

// Chain maintains an append-only ledger of blocks.
type Chain struct {
	mu        sync.RWMutex
	blocks    []Block
	validator string
	secret    string
}

// NewChain creates a chain with a genesis block.
func NewChain(validator, secret string) (*Chain, error) {
	if validator == "" {
		return nil, errors.New("validator id required")
	}

	if secret == "" {
		return nil, errors.New("validator secret required")
	}

	c := &Chain{
		blocks:    make([]Block, 0, 1),
		validator: validator,
		secret:    secret,
	}

	genesis := Block{
		Index:     0,
		Timestamp: time.Now().UTC(),
		PrevHash:  "",
		Events:    []Event{},
		Validator: validator,
	}

	hash, err := computeBlockHash(genesis)
	if err != nil {
		return nil, err
	}

	genesis.Hash = hash
	genesis.Signature = signHash(hash, secret)

	c.blocks = append(c.blocks, genesis)
	return c, nil
}

// Validator returns the validator identifier for the chain.
func (c *Chain) Validator() string {
	return c.validator
}

// AppendBlock appends a new block containing the provided events.
func (c *Chain) AppendBlock(events []Event) (Block, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(events) == 0 {
		return Block{}, errors.New("cannot append block with no events")
	}

	prev := c.blocks[len(c.blocks)-1]

	block := Block{
		Index:     len(c.blocks),
		Timestamp: time.Now().UTC(),
		PrevHash:  prev.Hash,
		Events:    deepCopyEvents(events),
		Validator: c.validator,
	}

	hash, err := computeBlockHash(block)
	if err != nil {
		return Block{}, err
	}
	block.Hash = hash
	block.Signature = signHash(hash, c.secret)

	c.blocks = append(c.blocks, block)
	return block, nil
}

// Blocks returns a copy of the blocks in the chain.
func (c *Chain) Blocks() []Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Block, len(c.blocks))
	copy(out, c.blocks)
	return out
}

// Head returns the most recent block in the chain.
func (c *Chain) Head() Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.blocks[len(c.blocks)-1]
}

// Length returns the number of blocks stored in the chain.
func (c *Chain) Length() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.blocks)
}

// Validate ensures that all hashes and indexes are consistent.
func (c *Chain) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.blocks) == 0 {
		return errors.New("chain has no blocks")
	}

	for i, block := range c.blocks {
		if block.Index != i {
			return fmt.Errorf("block %d has incorrect index %d", i, block.Index)
		}

		if i == 0 {
			if block.PrevHash != "" {
				return fmt.Errorf("genesis block should have empty prev hash")
			}
		} else {
			prev := c.blocks[i-1]
			if block.PrevHash != prev.Hash {
				return fmt.Errorf("block %d has invalid prev hash", i)
			}
		}

		hash, err := computeBlockHash(block)
		if err != nil {
			return err
		}
		if block.Hash != hash {
			return fmt.Errorf("block %d hash mismatch", i)
		}
		sig := signHash(hash, c.secret)
		if block.Signature != sig {
			return fmt.Errorf("block %d signature mismatch", i)
		}
	}

	return nil
}

// ReplaceBlocks replaces the chain's internal state with the provided blocks.
// This is primarily used when loading persisted data. The provided blocks must
// already contain valid hashes and signatures.
func (c *Chain) ReplaceBlocks(blocks []Block) error {
	if len(blocks) == 0 {
		return errors.New("cannot replace with empty block list")
	}

	// Validate before replacing to maintain integrity.
	if err := validateBlockSequence(blocks, c.secret); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.blocks = make([]Block, len(blocks))
	copy(c.blocks, blocks)
	return nil
}

func deepCopyEvents(events []Event) []Event {
	copied := make([]Event, len(events))
	for i, evt := range events {
		// Copy the payload map to avoid shared references.
		var payload map[string]interface{}
		if evt.Payload != nil {
			payload = make(map[string]interface{}, len(evt.Payload))
			for k, v := range evt.Payload {
				payload[k] = v
			}
		}

		copied[i] = Event{
			DeviceID: evt.DeviceID,
			Nonce:    evt.Nonce,
			TS:       evt.TS,
			Payload:  payload,
		}
	}
	return copied
}

func computeBlockHash(block Block) (string, error) {
	toHash := struct {
		Index     int       `json:"index"`
		Timestamp time.Time `json:"timestamp"`
		PrevHash  string    `json:"prev_hash"`
		Events    []Event   `json:"events"`
		Validator string    `json:"validator"`
	}{
		Index:     block.Index,
		Timestamp: block.Timestamp,
		PrevHash:  block.PrevHash,
		Events:    block.Events,
		Validator: block.Validator,
	}

	data, err := json.Marshal(toHash)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func signHash(hash string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(hash))
	return hex.EncodeToString(mac.Sum(nil))
}

func validateBlockSequence(blocks []Block, secret string) error {
	if len(blocks) == 0 {
		return errors.New("chain must contain at least one block")
	}

	for i, block := range blocks {
		if block.Index != i {
			return fmt.Errorf("block %d expected index %d got %d", i, i, block.Index)
		}

		if i == 0 {
			if block.PrevHash != "" {
				return fmt.Errorf("genesis block should have empty prev hash")
			}
		} else {
			prev := blocks[i-1]
			if block.PrevHash != prev.Hash {
				return fmt.Errorf("block %d previous hash mismatch", i)
			}
		}

		hash, err := computeBlockHash(block)
		if err != nil {
			return err
		}
		if block.Hash != hash {
			return fmt.Errorf("block %d hash mismatch", i)
		}
		sig := signHash(hash, secret)
		if block.Signature != sig {
			return fmt.Errorf("block %d signature mismatch", i)
		}
	}

	return nil
}

// Snapshot returns a deep copy of the underlying block slice.
func (c *Chain) Snapshot() []Block {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Block, len(c.blocks))
	for i, block := range c.blocks {
		// Create a copy of the block and its events to prevent
		// mutations from affecting the live chain state.
		bCopy := block
		bCopy.Events = deepCopyEvents(block.Events)
		out[i] = bCopy
	}
	return out
}
