package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/skychain/skychain/pkg/chain"
	"github.com/skychain/skychain/pkg/eventauth"
	"github.com/skychain/skychain/pkg/registry"
)

// Node orchestrates event ingestion, block production, and HTTP serving.
type Node struct {
	chain    *chain.Chain
	store    blockSink
	interval time.Duration
	registry *registry.DeviceRegistry

	pendingMu sync.Mutex
	pending   []chain.Event

	nonceMu   sync.Mutex
	lastNonce map[string]uint64

	startOnce sync.Once
	stopOnce  sync.Once
	stopChan  chan struct{}
}

type blockSink interface {
	PersistBlock(chain.Block) error
}

// NewNode creates a new SkyChain node.
func NewNode(c *chain.Chain, sink blockSink, interval time.Duration, reg *registry.DeviceRegistry) (*Node, error) {
	if c == nil {
		return nil, errors.New("chain required")
	}
	if sink == nil {
		return nil, errors.New("block store required")
	}
	if interval <= 0 {
		return nil, errors.New("interval must be positive")
	}
	if reg == nil {
		return nil, errors.New("device registry required")
	}

	return &Node{
		chain:     c,
		store:     sink,
		interval:  interval,
		registry:  reg,
		pending:   make([]chain.Event, 0),
		lastNonce: make(map[string]uint64),
		stopChan:  make(chan struct{}),
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

	valid := make([]chain.Event, 0, len(events))
	for _, evt := range events {
		if err := n.verifyEvent(evt); err != nil {
			log.Printf("discarding event from %s: %v", evt.DeviceID, err)
			continue
		}
		valid = append(valid, evt)
	}
	if len(valid) == 0 {
		return
	}

	block, err := n.chain.AppendBlock(valid)
	if err != nil {
		log.Printf("append block: %v", err)
		return
	}

	if err := n.store.PersistBlock(block); err != nil {
		log.Printf("persist block: %v", err)
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
	mux.HandleFunc("/proof", n.handleProof)
	mux.HandleFunc("/health", n.handleHealth)
	return mux
}

func (n *Node) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	var req eventRequest
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	evt, err := n.validateIncomingEvent(req)
	if err != nil {
		var verr *eventValidationError
		if errors.As(err, &verr) {
			http.Error(w, verr.message, verr.status)
			return
		}
		log.Printf("event validation failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	txid, err := chain.EventHash(evt)
	if err != nil {
		log.Printf("compute event hash: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	n.AddEvent(evt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "queued",
		"pending": n.Pending(),
		"txid":    txid,
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

func (n *Node) handleProof(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	txid := r.URL.Query().Get("txid")
	if txid == "" {
		http.Error(w, "txid query param is required", http.StatusBadRequest)
		return
	}

	blocks := n.chain.Blocks()
	for _, blk := range blocks {
		proof, err := chain.BuildProofForBlock(blk, txid)
		if err != nil {
			if errors.Is(err, chain.ErrTxNotFound) {
				continue
			}
			log.Printf("build proof: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		respondJSON(w, proof)
		return
	}

	http.Error(w, "transaction not found", http.StatusNotFound)
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

type eventRequest struct {
	DeviceID  string                 `json:"device_id"`
	Nonce     json.RawMessage        `json:"nonce"`
	TS        string                 `json:"ts"`
	Payload   map[string]interface{} `json:"payload"`
	Signature string                 `json:"sig"`
}

type eventValidationError struct {
	status  int
	message string
}

func (e *eventValidationError) Error() string {
	return e.message
}

func (n *Node) validateIncomingEvent(req eventRequest) (chain.Event, error) {
	if req.DeviceID == "" {
		return chain.Event{}, &eventValidationError{status: http.StatusBadRequest, message: "device_id is required"}
	}
	nonce, err := parseNonce(req.Nonce)
	if err != nil {
		return chain.Event{}, &eventValidationError{status: http.StatusBadRequest, message: err.Error()}
	}

	eventTime := time.Now().UTC()
	if req.TS != "" {
		parsed, err := time.Parse(time.RFC3339Nano, req.TS)
		if err != nil {
			return chain.Event{}, &eventValidationError{status: http.StatusBadRequest, message: "ts must be RFC3339 timestamp"}
		}
		eventTime = parsed
	}
	if req.Signature == "" {
		return chain.Event{}, &eventValidationError{status: http.StatusBadRequest, message: "sig is required"}
	}

	pubKey, err := n.registry.PublicKey(req.DeviceID)
	if err != nil {
		if errors.Is(err, registry.ErrUnknownDevice) {
			return chain.Event{}, &eventValidationError{status: http.StatusForbidden, message: "device is not registered"}
		}
		return chain.Event{}, err
	}

	evt := chain.Event{
		DeviceID:  req.DeviceID,
		Nonce:     nonce,
		TS:        eventTime,
		Payload:   req.Payload,
		Signature: req.Signature,
	}
	if err := eventauth.Verify(evt, pubKey); err != nil {
		if errors.Is(err, eventauth.ErrInvalidSignature) {
			return chain.Event{}, &eventValidationError{status: http.StatusForbidden, message: "invalid signature"}
		}
		return chain.Event{}, err
	}
	if err := n.recordNonce(req.DeviceID, nonce); err != nil {
		return chain.Event{}, &eventValidationError{status: http.StatusConflict, message: err.Error()}
	}
	return evt, nil
}

func (n *Node) recordNonce(deviceID string, nonce uint64) error {
	n.nonceMu.Lock()
	defer n.nonceMu.Unlock()

	if prev, ok := n.lastNonce[deviceID]; ok && nonce <= prev {
		return fmt.Errorf("nonce %d must be greater than %d", nonce, prev)
	}
	n.lastNonce[deviceID] = nonce
	return nil
}

func parseNonce(raw json.RawMessage) (uint64, error) {
	if raw == nil {
		return 0, errors.New("nonce is required")
	}
	var num uint64
	if err := json.Unmarshal(raw, &num); err == nil {
		return num, nil
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if str == "" {
			return 0, errors.New("nonce is required")
		}
		val, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("nonce must be an integer: %w", err)
		}
		return val, nil
	}
	return 0, errors.New("nonce must be a number or numeric string")
}

func (n *Node) verifyEvent(evt chain.Event) error {
	pubKey, err := n.registry.PublicKey(evt.DeviceID)
	if err != nil {
		return err
	}
	return eventauth.Verify(evt, pubKey)
}
