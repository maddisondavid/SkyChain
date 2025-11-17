package node

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/skychain/skychain/pkg/chain"
	"github.com/skychain/skychain/pkg/registry"
)

// Node orchestrates event ingestion, block production, and HTTP serving.
type Node struct {
	chain       *chain.Chain
	storagePath string
	interval    time.Duration
	registry    *registry.DeviceRegistry

	pendingMu sync.Mutex
	pending   []chain.Event

	startOnce sync.Once
	stopOnce  sync.Once
	stopChan  chan struct{}
}

// NewNode creates a new SkyChain node.
func NewNode(c *chain.Chain, storagePath string, interval time.Duration, reg *registry.DeviceRegistry) (*Node, error) {
	if c == nil {
		return nil, errors.New("chain required")
	}
	if storagePath == "" {
		return nil, errors.New("storage path required")
	}
	if interval <= 0 {
		return nil, errors.New("interval must be positive")
	}
	if reg == nil {
		return nil, errors.New("device registry required")
	}

	return &Node{
		chain:       c,
		storagePath: storagePath,
		interval:    interval,
		registry:    reg,
		pending:     make([]chain.Event, 0),
		stopChan:    make(chan struct{}),
	}, nil
}

// Start begins the background block production loop.
func (n *Node) Start(ctx context.Context) {
	n.startOnce.Do(func() {
		go func() {
			if err := n.registry.Watch(ctx, log.Default()); err != nil {
				log.Printf("registry watcher stopped: %v", err)
			}
		}()
		go n.run(ctx)
	})
}

// Stop signals the background loop to exit.
func (n *Node) Stop() {
	n.stopOnce.Do(func() {
		close(n.stopChan)
	})
}

// AddEvent queues an event for inclusion in the next block.
func (n *Node) AddEvent(evt chain.Event) {
	n.pendingMu.Lock()
	defer n.pendingMu.Unlock()

	evt.Payload = copyPayload(evt.Payload)
	n.pending = append(n.pending, evt)
}

// Pending returns the number of pending events.
func (n *Node) Pending() int {
	n.pendingMu.Lock()
	defer n.pendingMu.Unlock()

	return len(n.pending)
}

func (n *Node) run(ctx context.Context) {
	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			n.flush()
			return
		case <-n.stopChan:
			n.flush()
			return
		case <-ticker.C:
			n.flush()
		}
	}
}

func (n *Node) flush() {
	events := n.popPending()
	if len(events) == 0 {
		return
	}

	block, err := n.chain.AppendBlock(events)
	if err != nil {
		log.Printf("append block: %v", err)
		return
	}

	if err := n.chain.SaveToFile(n.storagePath); err != nil {
		log.Printf("persist chain: %v", err)
		return
	}

	log.Printf("sealed block %d with %d event(s)", block.Index, len(block.Events))
}

func (n *Node) popPending() []chain.Event {
	n.pendingMu.Lock()
	defer n.pendingMu.Unlock()

	if len(n.pending) == 0 {
		return nil
	}

	events := make([]chain.Event, len(n.pending))
	copy(events, n.pending)
	n.pending = n.pending[:0]
	return events
}

func copyPayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return nil
	}
	cp := make(map[string]interface{}, len(payload))
	for k, v := range payload {
		cp[k] = v
	}
	return cp
}

// Handler returns an HTTP handler implementing the node's REST API.
func (n *Node) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/event", n.handleEvent)
	mux.HandleFunc("/chain", n.handleChain)
	mux.HandleFunc("/head", n.handleHead)
	mux.HandleFunc("/health", n.handleHealth)
	return mux
}

func (n *Node) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string                 `json:"device_id"`
		Nonce    string                 `json:"nonce"`
		TS       string                 `json:"ts"`
		Payload  map[string]interface{} `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	if req.DeviceID == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}
	if _, err := n.registry.Lookup(req.DeviceID); err != nil {
		http.Error(w, "device is not registered", http.StatusForbidden)
		return
	}

	eventTime := time.Now().UTC()
	if req.TS != "" {
		parsed, err := time.Parse(time.RFC3339Nano, req.TS)
		if err != nil {
			http.Error(w, "ts must be RFC3339 timestamp", http.StatusBadRequest)
			return
		}
		eventTime = parsed
	}

	evt := chain.Event{
		DeviceID: req.DeviceID,
		Nonce:    req.Nonce,
		TS:       eventTime,
		Payload:  req.Payload,
	}

	n.AddEvent(evt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "queued",
		"pending": n.Pending(),
	})
}

func (n *Node) handleChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	blocks := n.chain.Blocks()
	respondJSON(w, blocks)
}

func (n *Node) handleHead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	head := n.chain.Head()
	respondJSON(w, head)
}

func (n *Node) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	head := n.chain.Head()
	payload := map[string]any{
		"status":       "ok",
		"validator":    n.chain.Validator(),
		"blocks":       n.chain.Length(),
		"pending":      n.Pending(),
		"head_index":   head.Index,
		"head_time":    head.Timestamp,
		"last_updated": time.Now().UTC(),
	}
	respondJSON(w, payload)
}

func respondJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write response: %v", err)
	}
}
